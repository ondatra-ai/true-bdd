package validate

import (
	"context"
	"encoding/json"
	"errors"
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

// FixPromptData represents data needed for fix prompt generation templates.
type FixPromptData struct {
	Subject     any
	SubjectID   string
	FailedCheck checklist.ValidationResult
	ResultPath  string
	UserAnswers map[string]string // Answers from user (nil if first iteration)
	Iteration   int               // Current iteration number
	DocPaths    map[string]string // Maps doc key to file path (e.g., "product" -> "docs/product/product.yaml")
	// Structured is true when the resolved CLI enforces a result schema.
	Structured bool
}

// fixResultSchema mirrors checklist.GenerateResult: exactly one of the two
// branches carries content, which the engine re-checks after decoding.
const fixResultSchema = `{
  "type": "object",
  "properties": {
    "fix_prompt": {"type": "string"},
    "questions": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "id": {"type": "string"},
          "question": {"type": "string"},
          "context": {"type": "string"},
          "options": {"type": "array", "items": {"type": "string"}}
        },
        "required": ["id", "question", "context", "options"],
        "additionalProperties": false
      }
    }
  },
  "required": ["fix_prompt", "questions"],
  "additionalProperties": false
}`

// GenerateParams contains parameters for fix prompt generation.
type GenerateParams struct {
	Subject     any
	SubjectID   string
	FailedCheck checklist.ValidationResult // Single failed check to generate fix for
	TmpDir      string
	UserAnswers map[string]string // Answers from previous clarification round (nil on first call)
	Iteration   int               // Current iteration (1-based, for logging/file naming)
}

// FixPromptGenerator generates complete, actionable fix prompts for failed checklist validations.
type FixPromptGenerator struct {
	aiClient     ports.AIPort
	config       *config.ViperConfig
	models       *provider.Registry
	modeFactory  *ai.ModeFactory
	docResolver  *docs.Resolver
	systemLoader *template.TemplateLoader[FixPromptData]
	userLoader   *template.TemplateLoader[FixPromptData]
}

// NewFixPromptGenerator creates a new fix prompt generator with config-based template paths.
func NewFixPromptGenerator(
	aiClient ports.AIPort,
	cfg *config.ViperConfig,
	models *provider.Registry,
) *FixPromptGenerator {
	systemTemplatePath := cfg.GetString("templates.prompts.fix_generator_system")
	userTemplatePath := cfg.GetString("templates.prompts.fix_generator")

	return NewFixPromptGeneratorWithPaths(aiClient, cfg, models, systemTemplatePath, userTemplatePath)
}

// NewFixPromptGeneratorWithPaths creates a new fix prompt generator with explicit template paths.
func NewFixPromptGeneratorWithPaths(
	aiClient ports.AIPort,
	cfg *config.ViperConfig,
	models *provider.Registry,
	systemPath, userPath string,
) *FixPromptGenerator {
	return &FixPromptGenerator{
		aiClient:     aiClient,
		config:       cfg,
		models:       models,
		modeFactory:  ai.NewModeFactory(cfg),
		docResolver:  docs.NewResolver(cfg),
		systemLoader: template.NewTemplateLoader[FixPromptData](systemPath),
		userLoader:   template.NewTemplateLoader[FixPromptData](userPath),
	}
}

// Generate creates a fix prompt OR returns questions if clarification is needed.
func (g *FixPromptGenerator) Generate(
	ctx context.Context,
	params GenerateParams,
) (checklist.GenerateResult, error) {
	promptIndex := params.FailedCheck.PromptIndex
	if promptIndex == 0 {
		slog.Warn("FailedCheck has no PromptIndex, skipping fix generation")

		return checklist.GenerateResult{}, nil
	}

	iteration := params.Iteration
	if iteration == 0 {
		iteration = 1
	}

	g.logGenerationStart(params, promptIndex, iteration)

	resultPath := fmt.Sprintf("%s/%02d-%s-fix-prompts.md", params.TmpDir, promptIndex, sanitizeID(params.SubjectID))
	promptData := g.buildPromptData(params, resultPath, iteration)

	response, structured, err := g.executeAIGeneration(ctx, params, promptData, promptIndex, iteration)
	if err != nil {
		return checklist.GenerateResult{}, err
	}

	return g.parseAndSaveResponse(response, resultPath, structured)
}

func (g *FixPromptGenerator) logGenerationStart(params GenerateParams, promptIndex, iteration int) {
	slog.Info("Generating fix prompt",
		"subjectID", params.SubjectID,
		"promptIndex", promptIndex,
		"section", params.FailedCheck.SectionPath,
		"iteration", iteration,
		"hasUserAnswers", len(params.UserAnswers) > 0,
	)
}

func (g *FixPromptGenerator) buildPromptData(params GenerateParams, resultPath string, iteration int) FixPromptData {
	// Resolve doc keys to file paths
	docPaths := g.resolveDocPaths(params.FailedCheck.Docs)

	// Same record the evaluator emits, for the same reason: the fix turn
	// reads its own document set, and the report names it when saying
	// what this turn was working from.
	slog.Info("Prompt documents resolved",
		"subjectID", params.SubjectID,
		"promptIndex", params.FailedCheck.PromptIndex,
		"section", params.FailedCheck.SectionPath,
		"docs", docPaths,
	)

	return FixPromptData{
		Subject:     params.Subject,
		SubjectID:   params.SubjectID,
		FailedCheck: params.FailedCheck,
		ResultPath:  resultPath,
		UserAnswers: params.UserAnswers,
		Iteration:   iteration,
		DocPaths:    docPaths,
	}
}

// resolveDocPaths converts doc keys to file paths. Same invariant as
// loadRequestedDocs (checklist_evaluator.go): the runner already refused an
// unsatisfiable doc, so a failure here is a wiring bug — skip, don't point the fix prompt at nothing.
func (g *FixPromptGenerator) resolveDocPaths(docKeys []string) map[string]string {
	docPaths := make(map[string]string, len(docKeys))

	for _, key := range docKeys {
		filePath, err := g.docResolver.Resolve(key)
		if err != nil {
			slog.Warn("Skipping unsatisfiable document", "key", key, "error", err)

			continue
		}

		docPaths[key] = filePath
	}

	return docPaths
}

func (g *FixPromptGenerator) executeAIGeneration(
	ctx context.Context,
	params GenerateParams,
	promptData FixPromptData,
	promptIndex, iteration int,
) (string, bool, error) {
	// The tier travels on the failed check — the evaluator resolved it
	// while the prompt was still in scope. Resolved first: it decides
	// which answer contract the templates render.
	model, err := g.models.ResolveRole(provider.RoleFix, params.FailedCheck.FixModelTier)
	if err != nil {
		return "", false, pkgerrors.ErrResolveModelTierFailed("fix generation", err)
	}

	promptData.Structured = ai.SupportsResultSchema(model.CLI)

	systemPrompt, err := g.systemLoader.LoadTemplate(promptData)
	if err != nil {
		return "", false, pkgerrors.ErrLoadChecklistSystemPromptFailed(err)
	}

	userPrompt, err := g.userLoader.LoadTemplate(promptData)
	if err != nil {
		return "", false, pkgerrors.ErrLoadChecklistUserPromptFailed(err)
	}

	suffix := fmt.Sprintf("fix-iter%d", iteration)
	g.savePromptFile(params.TmpDir, params.SubjectID, promptIndex, suffix+"-system", systemPrompt)
	g.savePromptFile(params.TmpDir, params.SubjectID, promptIndex, suffix+"-user", userPrompt)

	mode := g.modeFactory.GetThinkMode()

	schema := ""
	if promptData.Structured {
		schema = fixResultSchema
	}

	response, err := g.aiClient.ExecutePromptWithSystem(
		ctx, provider.RoleFix, systemPrompt, userPrompt, model, mode, schema,
	)
	if err != nil {
		return "", false, pkgerrors.ErrChecklistAIEvaluationFailed(err)
	}

	g.savePromptFile(params.TmpDir, params.SubjectID, promptIndex, suffix+"-response", response)

	return response, promptData.Structured, nil
}

func (g *FixPromptGenerator) parseAndSaveResponse(
	response, resultPath string, structured bool,
) (checklist.GenerateResult, error) {
	if structured {
		return g.decodeStructuredResult(response, resultPath)
	}

	if g.hasQuestions(response) {
		questions, parseErr := g.parseQuestions(response)
		if parseErr != nil {
			slog.Error(enginelog.MsgAnswerUnusable, "path", resultPath, "error", parseErr)

			return checklist.GenerateResult{}, pkgerrors.ErrFixGeneratorQuestionsDidNotParse(parseErr)
		}

		slog.Info("AI needs clarification", "questionCount", len(questions))

		return checklist.GenerateResult{Questions: questions}, nil
	}

	// An empty result here used to reach the caller as CellFailedNoFix, which
	// says "the checklist has no F: for this cell" — a different fact.
	fixPrompt := g.extractFixPrompt(response, resultPath)
	if fixPrompt == "" {
		slog.Error(enginelog.MsgAnswerUnusable, "path", resultPath)

		return checklist.GenerateResult{}, pkgerrors.ErrFixGeneratorEmptyAnswer(resultPath)
	}

	g.saveFixPrompt(resultPath, fixPrompt)

	return checklist.GenerateResult{FixPrompt: fixPrompt}, nil
}

// saveFixPrompt keeps the per-turn artifact the run dir carries.
func (g *FixPromptGenerator) saveFixPrompt(resultPath, fixPrompt string) {
	err := disk.Write(resultPath, []byte(fixPrompt), disk.Shared)
	if err != nil {
		slog.Warn("Failed to save fix prompt file", "path", resultPath, "error", err)
	} else {
		slog.Info("Fix prompt saved", "file", resultPath)
	}
}

// decodeStructuredResult reads a schema-enforced fix answer. Exactly one
// of the two branches carries content — the schema cannot express that,
// so the engine checks it here.
func (g *FixPromptGenerator) decodeStructuredResult(
	response, resultPath string,
) (checklist.GenerateResult, error) {
	var decoded struct {
		FixPrompt string                      `json:"fix_prompt"`
		Questions []checklist.ClarifyQuestion `json:"questions"`
	}

	err := json.Unmarshal([]byte(response), &decoded)
	if err != nil {
		slog.Error(enginelog.MsgAnswerUnusable, "path", resultPath, "error", err)

		return checklist.GenerateResult{}, pkgerrors.ErrFixGeneratorQuestionsDidNotParse(err)
	}

	fixPrompt := strings.TrimSpace(decoded.FixPrompt)
	if (fixPrompt == "") == (len(decoded.Questions) == 0) {
		slog.Error(enginelog.MsgAnswerUnusable, "path", resultPath)

		return checklist.GenerateResult{}, pkgerrors.ErrFixGeneratorEmptyAnswer(resultPath)
	}

	if len(decoded.Questions) > 0 {
		slog.Info("AI needs clarification", "questionCount", len(decoded.Questions))

		return checklist.GenerateResult{Questions: decoded.Questions}, nil
	}

	g.saveFixPrompt(resultPath, fixPrompt)

	return checklist.GenerateResult{FixPrompt: fixPrompt}, nil
}

// extractFixPrompt extracts content between FILE_START and FILE_END markers.
func (g *FixPromptGenerator) extractFixPrompt(response, path string) string {
	return ExtractFileContent(response, path)
}

// savePromptFile saves a prompt to a file in the tmp directory with naming convention.
func (g *FixPromptGenerator) savePromptFile(tmpDir, storyID string, promptIndex int, suffix, content string) {
	if tmpDir == "" {
		return
	}

	// Follow naming convention: XX-<storyID>-<suffix>.txt
	filePath := fmt.Sprintf("%s/%02d-%s-%s.txt", tmpDir, promptIndex, sanitizeID(storyID), suffix)

	err := disk.Write(filePath, []byte(content), disk.Shared)
	if err != nil {
		slog.Warn("Failed to save prompt file", "error", err)
	} else {
		slog.Info("Prompt saved", "file", filePath)
	}
}

const (
	questionsStartMarker = "=== QUESTIONS_START ==="
	questionsEndMarker   = "=== QUESTIONS_END ==="
)

var (
	errQuestionsStartMarkerNotFound = errors.New("no questions start marker found")
	errQuestionsEndMarkerNotFound   = errors.New("no questions end marker found")
)

// hasQuestions checks if response contains questions.
func (g *FixPromptGenerator) hasQuestions(response string) bool {
	return strings.Contains(response, questionsStartMarker)
}

// StripMarkdownCodeFences removes leading ```yaml/``` fences from YAML content.
func StripMarkdownCodeFences(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) >= 2 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
		// Remove first line (```yaml or ```)
		lines = lines[1:]
		// Remove last line if it's a closing fence
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			lines = lines[:len(lines)-1]
		}
	}

	return strings.Join(lines, "\n")
}

// parseQuestions extracts questions from response.
func (g *FixPromptGenerator) parseQuestions(response string) ([]checklist.ClarifyQuestion, error) {
	startIdx := strings.Index(response, questionsStartMarker)
	if startIdx == -1 {
		return nil, errQuestionsStartMarkerNotFound
	}

	contentStart := startIdx + len(questionsStartMarker)
	endIdx := strings.Index(response[contentStart:], questionsEndMarker)

	if endIdx == -1 {
		return nil, errQuestionsEndMarkerNotFound
	}

	yamlContent := StripMarkdownCodeFences(strings.TrimSpace(response[contentStart : contentStart+endIdx]))

	// Parse YAML structure
	var wrapper struct {
		Questions []checklist.ClarifyQuestion `yaml:"questions"`
	}

	err := yaml.Unmarshal([]byte(yamlContent), &wrapper)
	if err != nil {
		return nil, fmt.Errorf("failed to parse questions YAML: %w", err)
	}

	return wrapper.Questions, nil
}
