package validate

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"bdd-cli/src/adapters/ai"
	"bdd-cli/src/internal/domain/models/checklist"
	"bdd-cli/src/internal/domain/ports"
	"bdd-cli/src/internal/infrastructure/config"
	"bdd-cli/src/internal/infrastructure/docs"
	"bdd-cli/src/internal/infrastructure/template"
	pkgerrors "bdd-cli/src/internal/pkg/errors"
)

const (
	filePermissions = 0o644 // File permissions for saved prompts
)

// getDocKeyToConfigPath returns the mapping of doc keys to bdd-cli.yaml config paths.
func getDocKeyToConfigPath() map[string]string {
	return map[string]string{
		"architecture":          "documents.architecture",
		"frontend_architecture": "documents.frontend_architecture",
		"coding_standards":      "documents.coding_standards",
		"source_tree":           "documents.source_tree",
		"tech_stack":            "documents.tech_stack",
		"prd":                   "documents.prd",
		"terms":                 "documents.terms",
		"architecture_yaml":     "documents.architecture_yaml",
		"bdd_guidelines":        "documents.bdd_guidelines",
	}
}

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
	modeFactory  *ai.ModeFactory
	systemLoader *template.TemplateLoader[ChecklistPromptData]
	userLoader   *template.TemplateLoader[ChecklistPromptData]
	tmpDir       string
	subjectID    string
}

// NewChecklistEvaluator creates a new checklist evaluator with config-based template paths.
func NewChecklistEvaluator(aiClient ports.AIPort, cfg *config.ViperConfig) *ChecklistEvaluator {
	systemTemplatePath := cfg.GetString("templates.prompts.checklist_system")
	userTemplatePath := cfg.GetString("templates.prompts.checklist")

	return NewChecklistEvaluatorWithPaths(aiClient, cfg, systemTemplatePath, userTemplatePath)
}

// NewChecklistEvaluatorWithPaths creates a new checklist evaluator with explicit template paths.
func NewChecklistEvaluatorWithPaths(
	aiClient ports.AIPort,
	cfg *config.ViperConfig,
	systemPath, userPath string,
) *ChecklistEvaluator {
	return &ChecklistEvaluator{
		aiClient:     aiClient,
		config:       cfg,
		modeFactory:  ai.NewModeFactory(cfg),
		systemLoader: template.NewTemplateLoader[ChecklistPromptData](systemPath),
		userLoader:   template.NewTemplateLoader[ChecklistPromptData](userPath),
	}
}

// Evaluate evaluates all prompts against the given subject.
func (e *ChecklistEvaluator) Evaluate(
	ctx context.Context,
	subject any,
	subjectID, subjectTitle string,
	prompts []checklist.PromptWithContext,
	tmpDir string,
) (*checklist.ChecklistReport, error) {
	return e.evaluatePrompts(ctx, subject, subjectID, subjectTitle, prompts, tmpDir, false)
}

// EvaluateUntilFailure evaluates prompts sequentially and stops at the first FAIL status.
// This is used for the iterative fix workflow where we want to fix one issue at a time.
func (e *ChecklistEvaluator) EvaluateUntilFailure(
	ctx context.Context,
	subject any,
	subjectID, subjectTitle string,
	prompts []checklist.PromptWithContext,
	tmpDir string,
) (*checklist.ChecklistReport, error) {
	return e.evaluatePrompts(ctx, subject, subjectID, subjectTitle, prompts, tmpDir, true)
}

// evaluatePrompts is the shared implementation for Evaluate and EvaluateUntilFailure.
func (e *ChecklistEvaluator) evaluatePrompts(
	ctx context.Context,
	subject any,
	subjectID, subjectTitle string,
	prompts []checklist.PromptWithContext,
	tmpDir string,
	stopOnFailure bool,
) (*checklist.ChecklistReport, error) {
	e.tmpDir = tmpDir
	e.subjectID = subjectID

	report := &checklist.ChecklistReport{
		SubjectID:    subjectID,
		SubjectTitle: subjectTitle,
		Results:      make([]checklist.ValidationResult, 0, len(prompts)),
	}

	for promptIndex, promptCtx := range prompts {
		slog.Info("Evaluating prompt",
			"index", promptIndex+1,
			"total", len(prompts),
			"section", promptCtx.GetFullSectionPath(),
		)

		result, err := e.evaluatePrompt(ctx, subject, subjectID, promptCtx, promptIndex+1)
		if err != nil {
			slog.Error("Failed to evaluate prompt", "error", err)
			// Continue with other prompts, mark this one as failed
			result = checklist.ValidationResult{
				SectionPath:  promptCtx.GetFullSectionPath(),
				Question:     promptCtx.Prompt.Question,
				ActualAnswer: "ERROR: " + err.Error(),
				Status:       checklist.StatusFail,
				Rationale:    promptCtx.Prompt.Rationale,
				PromptIndex:  promptIndex + 1,
				Docs:         promptCtx.GetEffectiveDocs(),
			}
		}

		report.Results = append(report.Results, result)

		// Stop at first failure if requested
		if stopOnFailure && result.Status == checklist.StatusFail {
			slog.Info("Stopping evaluation at first failure",
				"section", promptCtx.GetFullSectionPath(),
				"promptIndex", promptIndex+1,
			)

			break
		}
	}

	report.CalculateSummary()

	return report, nil
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
	safeSectionPath := strings.ReplaceAll(sectionPath, "/", "-")
	resultPath := fmt.Sprintf("%s/%02d-%s-checklist-%s-result.yaml",
		e.tmpDir, promptIndex, e.subjectID, safeSectionPath)

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

	response, err := e.aiClient.ExecutePromptWithSystem(ctx, systemPrompt, userPrompt, "", mode)
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
func (e *ChecklistEvaluator) loadRequestedDocs(keys []string) map[string]*docs.ArchitectureDoc {
	result := make(map[string]*docs.ArchitectureDoc, len(keys))
	docKeyMapping := getDocKeyToConfigPath()

	for _, key := range keys {
		configPath, ok := docKeyMapping[key]
		if !ok {
			slog.Warn("Unknown document key, skipping", "key", key)

			continue
		}

		filePath := e.config.GetString(configPath)
		if filePath == "" {
			slog.Warn("Document path not configured, skipping", "key", key, "configPath", configPath)

			continue
		}

		result[key] = &docs.ArchitectureDoc{
			FilePath: filePath,
		}
	}

	return result
}

// savePromptFile saves a prompt to a file in the tmp directory.
func (e *ChecklistEvaluator) savePromptFile(sectionPath string, promptIndex int, suffix, content string) {
	if e.tmpDir == "" {
		return
	}

	// Replace slashes in section path with dashes for filename
	safeSectionPath := strings.ReplaceAll(sectionPath, "/", "-")
	filePath := fmt.Sprintf("%s/%02d-%s-checklist-%s-%s.txt", e.tmpDir, promptIndex, e.subjectID, safeSectionPath, suffix)

	err := os.WriteFile(filePath, []byte(content), filePermissions)
	if err != nil {
		slog.Warn("Failed to save prompt file", "error", err)
	} else {
		slog.Info("Prompt saved", "file", filePath)
	}
}
