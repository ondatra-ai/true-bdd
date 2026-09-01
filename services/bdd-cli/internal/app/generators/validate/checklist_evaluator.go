package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/enginelog"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/ports"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/template"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

const ()

// ChecklistPromptData represents data needed for checklist validation prompts.
type ChecklistPromptData struct {
	Subject      any
	SubjectID    string
	SubjectTitle string
	Question     string
	Rationale    string
	ResultPath   string
	Docs         map[string]*docs.ArchitectureDoc
	FixTemplate  string // Template for generating fix prompt when validation fails
	// Structured is true when the resolved CLI enforces a result schema,
	// so the template asks for a JSON object instead of a FILE block.
	Structured bool
}

// checklistResultSchema is the answer shape a schema-bearing CLI is held
// to. It mirrors resultYAML: pass/fail is the universal contract.
const checklistResultSchema = `{
  "type": "object",
  "properties": {
    "answer": {"type": "string", "enum": ["pass", "fail"]},
    "context": {"type": "array", "items": {"type": "string"}},
    "fix_prompt": {"type": "string"}
  },
  "required": ["answer", "context"],
  "additionalProperties": false
}`

// ChecklistEvaluator evaluates subjects against validation prompts using AI.
type ChecklistEvaluator struct {
	aiClient     ports.AIPort
	config       *config.ViperConfig
	models       *provider.Registry
	modeFactory  *ai.ModeFactory
	docResolver  *docs.Resolver
	systemLoader *template.TemplateLoader[ChecklistPromptData]
	userLoader   *template.TemplateLoader[ChecklistPromptData]
	tmpDir       string
	subjectID    string
}

// NewChecklistEvaluator creates a new checklist evaluator with config-based template paths.
func NewChecklistEvaluator(
	aiClient ports.AIPort,
	cfg *config.ViperConfig,
	models *provider.Registry,
) *ChecklistEvaluator {
	systemTemplatePath := cfg.GetString("templates.prompts.checklist_system")
	userTemplatePath := cfg.GetString("templates.prompts.checklist")

	return NewChecklistEvaluatorWithPaths(aiClient, cfg, models, systemTemplatePath, userTemplatePath)
}

// NewChecklistEvaluatorWithPaths creates a new checklist evaluator with explicit template paths.
func NewChecklistEvaluatorWithPaths(
	aiClient ports.AIPort,
	cfg *config.ViperConfig,
	models *provider.Registry,
	systemPath, userPath string,
) *ChecklistEvaluator {
	return &ChecklistEvaluator{
		aiClient:     aiClient,
		config:       cfg,
		models:       models,
		modeFactory:  ai.NewModeFactory(cfg),
		docResolver:  docs.NewResolver(cfg),
		systemLoader: template.NewTemplateLoader[ChecklistPromptData](systemPath),
		userLoader:   template.NewTemplateLoader[ChecklistPromptData](userPath),
	}
}

// EvaluateOne evaluates a single prompt against the subject and returns the
// full ValidationResult, not just pass/fail, so the cell's `genFix` closure
// can read it via shared closure state. promptIndex must be 1-based to match the tmp-file naming convention.
func (e *ChecklistEvaluator) EvaluateOne(
	ctx context.Context,
	subject any,
	subjectID string,
	promptCtx checklist.PromptWithContext,
	tmpDir string,
	promptIndex int,
) (checklist.ValidationResult, error) {
	e.tmpDir = tmpDir
	e.subjectID = subjectID

	return e.evaluatePrompt(ctx, subject, subjectID, promptCtx, promptIndex)
}

// evaluatePrompt evaluates a single prompt against the subject.
func (e *ChecklistEvaluator) evaluatePrompt(
	ctx context.Context,
	subject any,
	subjectID string,
	promptCtx checklist.PromptWithContext,
	promptIndex int,
) (checklist.ValidationResult, error) {
	// Load requested documents for this prompt (uses prompt-specific or defaults)
	requestedDocs := e.loadRequestedDocs(promptCtx.GetEffectiveDocs())

	// Build result file path for FILE_START/FILE_END pattern
	sectionPath := promptCtx.GetFullSectionPath()
	safeSectionPath := sanitizeID(sectionPath)
	resultPath := fmt.Sprintf("%s/%02d-%s-checklist-%s-result.yaml",
		e.tmpDir, promptIndex, sanitizeID(e.subjectID), safeSectionPath)

	e.logResolvedDocs(promptIndex, sectionPath, requestedDocs)

	// Tier resolution comes FIRST: whether the CLI can enforce a result
	// schema decides which answer contract the templates render.
	model, err := e.models.ResolveRole(provider.RolePrompt, promptCtx.EffectiveModelTier())
	if err != nil {
		return checklist.ValidationResult{}, pkgerrors.ErrResolveModelTierFailed("validation", err)
	}

	promptData := ChecklistPromptData{
		Subject:     subject,
		SubjectID:   subjectID,
		Question:    promptCtx.Prompt.Question,
		Rationale:   promptCtx.Prompt.Rationale,
		ResultPath:  resultPath,
		Docs:        requestedDocs,
		FixTemplate: promptCtx.Prompt.FixTemplate,
		Structured:  ai.SupportsResultSchema(model.CLI),
	}

	systemPrompt, userPrompt, err := e.renderPrompts(promptData)
	if err != nil {
		return checklist.ValidationResult{}, err
	}

	// Save prompts to tmp for debugging
	e.savePromptFile(sectionPath, promptIndex, "system", systemPrompt)
	e.savePromptFile(sectionPath, promptIndex, "user", userPrompt)

	// Use think mode - allows Read, Glob, Grep tools for accessing reference docs
	mode := e.modeFactory.GetThinkMode()

	schema := ""
	if promptData.Structured {
		schema = checklistResultSchema
	}

	response, err := e.aiClient.ExecutePromptWithSystem(
		ctx, provider.RolePrompt, systemPrompt, userPrompt, model, mode, schema,
	)
	if err != nil {
		return checklist.ValidationResult{}, pkgerrors.ErrChecklistAIEvaluationFailed(err)
	}

	// Save response to tmp
	e.savePromptFile(sectionPath, promptIndex, "response", response)

	parsedResult, status, err := e.gradeAnswer(response, resultPath, promptData.Structured)
	if err != nil {
		return checklist.ValidationResult{}, err
	}

	// Only include fix prompt if validation failed
	fixPrompt := ""
	if status == checklist.StatusFail && parsedResult.FixPrompt != "" {
		fixPrompt = parsedResult.FixPrompt
	}

	return checklist.ValidationResult{
		SectionPath:  promptCtx.GetFullSectionPath(),
		Question:     promptCtx.Prompt.Question,
		ActualAnswer: parsedResult.Answer,
		Context:      parsedResult.Context,
		Status:       status,
		Rationale:    promptCtx.Prompt.Rationale,
		FixPrompt:    fixPrompt,
		PromptIndex:  promptIndex,
		Docs:         promptCtx.GetEffectiveDocs(),
		// Carried forward because the fix generator and applier only
		// ever see a ValidationResult, never the prompt — same channel
		// Docs already travels on.
		FixModelTier:   promptCtx.EffectiveFixTier(),
		ApplyModelTier: promptCtx.EffectiveApplyTier(),
	}, nil
}

// renderPrompts loads both templates from one data value. The system
// template used to render from a zero value, which would now hide the
// answer contract from half the prompt.
func (e *ChecklistEvaluator) renderPrompts(data ChecklistPromptData) (string, string, error) {
	systemPrompt, err := e.systemLoader.LoadTemplate(data)
	if err != nil {
		return "", "", pkgerrors.ErrLoadChecklistSystemPromptFailed(err)
	}

	userPrompt, err := e.userLoader.LoadTemplate(data)
	if err != nil {
		return "", "", pkgerrors.ErrLoadChecklistUserPromptFailed(err)
	}

	return systemPrompt, userPrompt, nil
}

// gradeAnswer reads the model's answer and holds it to the universal
// pass/fail contract. Both refusals are infrastructure errors: grading an
// unreadable answer reports a verdict the model never gave.
func (e *ChecklistEvaluator) gradeAnswer(
	response, resultPath string, structured bool,
) (ParsedResult, checklist.Status, error) {
	parsed, err := e.readAnswer(response, resultPath, structured)
	if err != nil {
		slog.Error(enginelog.MsgAnswerUnusable, "path", resultPath, "error", err)

		return ParsedResult{}, checklist.StatusFail, err
	}

	status, err := canonicalStatus(parsed.Answer, resultPath)
	if err != nil {
		slog.Error(enginelog.MsgAnswerUnusable, "path", resultPath, "error", err)

		return ParsedResult{}, checklist.StatusFail, err
	}

	return parsed, status, nil
}

// resultYAML represents the structure of the result file. The Answer field
// uses yaml.Node so it can hold either a scalar (integer, yes/no, percentage)
// or a mapping (violation map keyed by AC id).
type resultYAML struct {
	Answer    yaml.Node `yaml:"answer"`
	Context   []string  `yaml:"context,omitempty"`
	FixPrompt string    `yaml:"fix_prompt,omitempty"`
}

// ParsedResult contains the parsed answer and optional fix prompt.
type ParsedResult struct {
	Answer    string
	Context   []string
	FixPrompt string
}

// readAnswer decodes the answer under whichever contract the turn ran on.
// A schema-bearing turn is parsed strictly: falling back to the delimited
// scrape would mask the failure the schema exists to prevent.
func (e *ChecklistEvaluator) readAnswer(response, path string, structured bool) (ParsedResult, error) {
	if !structured {
		return e.parseResultFile(response, path)
	}

	var decoded struct {
		Answer    string   `json:"answer"`
		Context   []string `json:"context"`
		FixPrompt string   `json:"fix_prompt"`
	}

	err := json.Unmarshal([]byte(response), &decoded)
	if err != nil {
		return ParsedResult{}, pkgerrors.ErrChecklistAnswerDidNotParse(path, err)
	}

	e.saveResultArtifact(path, response)

	return ParsedResult{
		Answer:    decoded.Answer,
		Context:   decoded.Context,
		FixPrompt: strings.TrimSpace(decoded.FixPrompt),
	}, nil
}

// saveResultArtifact keeps the per-turn result file the run dir carries.
func (e *ChecklistEvaluator) saveResultArtifact(path, content string) {
	err := disk.Write(path, []byte(content), disk.Shared)
	if err != nil {
		slog.Warn("Failed to save result file", "path", path, "error", err)
	} else {
		slog.Info("Result file saved", "file", path)
	}
}

// parseResultFile extracts FILE_START/FILE_END content from response, saves to
// file, and parses. A missing or undecodable answer is an infrastructure
// error: grading it as a fail fabricates a verdict the model never gave.
func (e *ChecklistEvaluator) parseResultFile(response, path string) (ParsedResult, error) {
	// Extract content between FILE_START and FILE_END markers
	content := ExtractFileContent(response, path)
	if content == "" {
		return ParsedResult{}, pkgerrors.ErrChecklistAnswerMissingBlock(path)
	}

	// Strip markdown code fences (```yaml ... ```) that some models add
	// inside the FILE_START/FILE_END block.
	content = stripMarkdownFences(content)

	// Save the extracted content to file
	err := disk.Write(path, []byte(content), disk.Shared)
	if err != nil {
		slog.Warn("Failed to save result file", "path", path, "error", err)
	} else {
		slog.Info("Result file saved", "file", path)
	}

	// Parse the YAML
	var result resultYAML

	err = yaml.Unmarshal([]byte(content), &result)
	if err != nil {
		return ParsedResult{}, pkgerrors.ErrChecklistAnswerDidNotParse(path, err)
	}

	return ParsedResult{
		Answer:    renderAnswerNode(&result.Answer),
		Context:   result.Context,
		FixPrompt: strings.TrimSpace(result.FixPrompt),
	}, nil
}

// canonicalStatus holds the answer to the universal pass/fail contract. An
// answer outside it used to grade StatusFail silently.
func canonicalStatus(answer, path string) (checklist.Status, error) {
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "pass":
		return checklist.StatusPass, nil
	case "fail":
		return checklist.StatusFail, nil
	default:
		return checklist.StatusFail, pkgerrors.ErrChecklistAnswerNotCanonical(path, answer)
	}
}

// renderAnswerNode converts the YAML answer node back to its text form. For
// scalars it returns the raw value ("5", "yes"); for mappings and sequences
// it preserves YAML block structure, matching what downstream display/comparison expects.
func renderAnswerNode(node *yaml.Node) string {
	if node == nil || node.Kind == 0 {
		return ""
	}

	if node.Kind == yaml.ScalarNode {
		return strings.TrimSpace(node.Value)
	}

	out, err := yaml.Marshal(node)
	if err != nil {
		slog.Warn("Failed to marshal answer node", "error", err)

		return ""
	}

	return strings.TrimRight(string(out), "\n")
}

// stripMarkdownFences removes leading/trailing markdown code fences
// (```yaml, ```yml, or plain ```) from a YAML payload — some models wrap
// answer blocks in fences even under FILE_START/FILE_END markers.
func stripMarkdownFences(content string) string {
	content = strings.TrimSpace(content)

	// Strip leading fence on its own line: ```yaml, ```yml, or ```
	if strings.HasPrefix(content, "```") {
		if idx := strings.Index(content, "\n"); idx >= 0 {
			content = content[idx+1:]
		} else {
			content = strings.TrimPrefix(content, "```")
		}
	}

	content = strings.TrimSpace(content)
	content = strings.TrimSuffix(content, "```")

	return strings.TrimSpace(content)
}

// loadRequestedDocs resolves document keys to file paths. By the time a
// walk reaches here the runner has already refused any run whose declared
// documents cannot be satisfied, so a failure is a wiring bug — log and skip, not a Read() for a missing path.
func (e *ChecklistEvaluator) loadRequestedDocs(keys []string) map[string]*docs.ArchitectureDoc {
	result := make(map[string]*docs.ArchitectureDoc, len(keys))

	for _, key := range keys {
		filePath, err := e.docResolver.Resolve(key)
		if err != nil {
			slog.Warn("Skipping unsatisfiable document", "key", key, "error", err)

			continue
		}

		result[key] = &docs.ArchitectureDoc{
			FilePath: filePath,
		}
	}

	return result
}

// logResolvedDocs records the other side of the comparison: loadRequestedDocs
// only logs failures, so without this the run keeps no trace of WHAT the
// subject was checked against — only that it was checked.
func (e *ChecklistEvaluator) logResolvedDocs(
	promptIndex int,
	sectionPath string,
	requestedDocs map[string]*docs.ArchitectureDoc,
) {
	slog.Info("Prompt documents resolved",
		"subjectID", e.subjectID,
		"promptIndex", promptIndex,
		"section", sectionPath,
		"docs", docFilePaths(requestedDocs),
	)
}

// docFilePaths flattens the resolved document set to key → path, the
// shape the run log records. The docs themselves carry template data the
// log has no use for.
func docFilePaths(resolved map[string]*docs.ArchitectureDoc) map[string]string {
	paths := make(map[string]string, len(resolved))
	for key, doc := range resolved {
		paths[key] = doc.FilePath
	}

	return paths
}

// savePromptFile saves a prompt to a file in the tmp directory.
func (e *ChecklistEvaluator) savePromptFile(sectionPath string, promptIndex int, suffix, content string) {
	if e.tmpDir == "" {
		return
	}

	// Flatten section path and subject id into safe filename segments.
	safeSectionPath := sanitizeID(sectionPath)
	filePath := fmt.Sprintf("%s/%02d-%s-checklist-%s-%s.txt",
		e.tmpDir, promptIndex, sanitizeID(e.subjectID), safeSectionPath, suffix)

	err := disk.Write(filePath, []byte(content), disk.Shared)
	if err != nil {
		slog.Warn("Failed to save prompt file", "error", err)
	} else {
		slog.Info("Prompt saved", "file", filePath)
	}
}
