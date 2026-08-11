package validate

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/src/adapters/ai"
	"github.com/ondatra-ai/true-bdd/src/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/src/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/src/internal/domain/ports"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/template"
	pkgerrors "github.com/ondatra-ai/true-bdd/src/internal/pkg/errors"
)

const (
	filePermissions = 0o644 // File permissions for saved prompts
)

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
}

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

// EvaluateOne evaluates a single prompt against the subject and returns
// the full ValidationResult. This is the per-cell primitive the engine
// `query` closure calls — returning the result (not just pass/fail) so
// the cell's `genFix` closure can read it via shared closure state.
//
// promptIndex must be 1-based to match the tmp-file naming convention.
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

	// Load system prompt template (uses cached loader)
	systemPrompt, err := e.systemLoader.LoadTemplate(ChecklistPromptData{})
	if err != nil {
		return checklist.ValidationResult{}, pkgerrors.ErrLoadChecklistSystemPromptFailed(err)
	}

	// Load user prompt template with data (uses cached loader)
	promptData := ChecklistPromptData{
		Subject:     subject,
		SubjectID:   subjectID,
		Question:    promptCtx.Prompt.Question,
		Rationale:   promptCtx.Prompt.Rationale,
		ResultPath:  resultPath,
		Docs:        requestedDocs,
		FixTemplate: promptCtx.Prompt.FixTemplate,
	}

	userPrompt, err := e.userLoader.LoadTemplate(promptData)
	if err != nil {
		return checklist.ValidationResult{}, pkgerrors.ErrLoadChecklistUserPromptFailed(err)
	}

	// Save prompts to tmp for debugging
	e.savePromptFile(sectionPath, promptIndex, "system", systemPrompt)
	e.savePromptFile(sectionPath, promptIndex, "user", userPrompt)

	// Use think mode - allows Read, Glob, Grep tools for accessing reference docs
	mode := e.modeFactory.GetThinkMode()

	// Tier resolution: this prompt's `model:`, else the checklist's
	// `prompt_model:`, else engine.default_prompt_model.
	model, err := e.models.ResolveRole(provider.RolePrompt, promptCtx.EffectiveModelTier())
	if err != nil {
		return checklist.ValidationResult{}, pkgerrors.ErrResolveModelTierFailed("validation", err)
	}

	response, err := e.aiClient.ExecutePromptWithSystem(
		ctx, provider.RolePrompt, systemPrompt, userPrompt, model, mode,
	)
	if err != nil {
		return checklist.ValidationResult{}, pkgerrors.ErrChecklistAIEvaluationFailed(err)
	}

	// Save response to tmp
	e.savePromptFile(sectionPath, promptIndex, "response", response)

	// Parse the answer from result file (extracted from FILE_START/FILE_END in response)
	parsedResult := e.parseResultFile(response, resultPath)

	// Universal pass/fail: AI emits `answer: pass` or `answer: fail`.
	status := checklist.StatusFail
	if strings.EqualFold(strings.TrimSpace(parsedResult.Answer), "pass") {
		status = checklist.StatusPass
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

// parseResultFile extracts FILE_START/FILE_END content from response, saves to file, and parses.
func (e *ChecklistEvaluator) parseResultFile(response, path string) ParsedResult {
	// Extract content between FILE_START and FILE_END markers
	content := ExtractFileContent(response, path)
	if content == "" {
		slog.Warn("No FILE_START/FILE_END content found in response", "path", path)

		return ParsedResult{}
	}

	// Strip markdown code fences (```yaml ... ```) that some models add
	// inside the FILE_START/FILE_END block.
	content = stripMarkdownFences(content)

	// Save the extracted content to file
	err := os.WriteFile(path, []byte(content), filePermissions)
	if err != nil {
		slog.Warn("Failed to save result file", "path", path, "error", err)
	} else {
		slog.Info("Result file saved", "file", path)
	}

	// Parse the YAML
	var result resultYAML

	err = yaml.Unmarshal([]byte(content), &result)
	if err != nil {
		slog.Warn("Failed to parse result YAML", "path", path, "error", err)

		return ParsedResult{}
	}

	return ParsedResult{
		Answer:    renderAnswerNode(&result.Answer),
		Context:   result.Context,
		FixPrompt: strings.TrimSpace(result.FixPrompt),
	}
}

// renderAnswerNode converts the YAML answer node back to its text form. For
// scalars this returns the raw value (e.g. "5", "yes"); for mappings and
// sequences it preserves the YAML block structure so downstream display and
// comparison logic see the same text the user would read in the result file.
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
// (```yaml, ```yml, or plain ```) from a YAML payload. Some models wrap
// answer blocks inside markdown fences even when the surrounding format
// is FILE_START/FILE_END markers; this normalizes that.
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

// loadRequestedDocs resolves document keys to file paths.
//
// By the time a walk reaches here the runner has already refused any
// run whose declared documents cannot be satisfied, so a failure is a
// wiring bug rather than a user-facing condition — log it and skip
// rather than injecting a Read() instruction for a path that is not
// there.
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

// logResolvedDocs records the other side of the comparison.
// loadRequestedDocs resolves every `docs:` key to a real path and only
// logs the failures, so without this record the run keeps no trace of
// WHAT the subject was checked against — only that it was checked.
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

	err := os.WriteFile(filePath, []byte(content), filePermissions)
	if err != nil {
		slog.Warn("Failed to save prompt file", "error", err)
	} else {
		slog.Info("Prompt saved", "file", filePath)
	}
}
