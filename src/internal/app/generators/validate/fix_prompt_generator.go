package validate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"bdd-cli/src/adapters/ai"
	"bdd-cli/src/internal/domain/models/checklist"
	"bdd-cli/src/internal/domain/ports"
	"bdd-cli/src/internal/infrastructure/config"
	"bdd-cli/src/internal/infrastructure/template"
	pkgerrors "bdd-cli/src/internal/pkg/errors"
)

const fixPromptFilePermissions = 0o644

// FixPromptData represents data needed for fix prompt generation templates.
type FixPromptData struct {
	Subject     any
	SubjectID   string
	FailedCheck checklist.ValidationResult
	ResultPath  string
	UserAnswers map[string]string // Answers from user (nil if first iteration)
	Iteration   int               // Current iteration number
	DocPaths    map[string]string // Maps doc key to file path (e.g., "prd" -> "docs/prd.md")
}

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
	modeFactory  *ai.ModeFactory
	systemLoader *template.TemplateLoader[FixPromptData]
	userLoader   *template.TemplateLoader[FixPromptData]
}

// NewFixPromptGenerator creates a new fix prompt generator with config-based template paths.
func NewFixPromptGenerator(aiClient ports.AIPort, cfg *config.ViperConfig) *FixPromptGenerator {
	systemTemplatePath := cfg.GetString("templates.prompts.fix_generator_system")
	userTemplatePath := cfg.GetString("templates.prompts.fix_generator")

	return NewFixPromptGeneratorWithPaths(aiClient, cfg, systemTemplatePath, userTemplatePath)
}

// NewFixPromptGeneratorWithPaths creates a new fix prompt generator with explicit template paths.
func NewFixPromptGeneratorWithPaths(
	aiClient ports.AIPort,
	cfg *config.ViperConfig,
	systemPath, userPath string,
) *FixPromptGenerator {
	return &FixPromptGenerator{
		aiClient:     aiClient,
		config:       cfg,
		modeFactory:  ai.NewModeFactory(cfg),
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

	resultPath := fmt.Sprintf("%s/%02d-%s-fix-prompts.md", params.TmpDir, promptIndex, params.SubjectID)
	promptData := g.buildPromptData(params, resultPath, iteration)

	response, err := g.executeAIGeneration(ctx, params, promptData, promptIndex, iteration)
	if err != nil {
		return checklist.GenerateResult{}, err
	}

	return g.parseAndSaveResponse(response, resultPath)
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

// resolveDocPaths converts doc keys to file paths using config.
func (g *FixPromptGenerator) resolveDocPaths(docKeys []string) map[string]string {
	docPaths := make(map[string]string, len(docKeys))
	docKeyMapping := getDocKeyToConfigPath()

	for _, key := range docKeys {
		configPath, ok := docKeyMapping[key]
		if !ok {
			continue
		}

		filePath := g.config.GetString(configPath)
		if filePath != "" {
			docPaths[key] = filePath
		}
	}

	return docPaths
}

func (g *FixPromptGenerator) executeAIGeneration(
	ctx context.Context,
	params GenerateParams,
	promptData FixPromptData,
	promptIndex, iteration int,
) (string, error) {
	systemPrompt, err := g.systemLoader.LoadTemplate(promptData)
	if err != nil {
		return "", pkgerrors.ErrLoadChecklistSystemPromptFailed(err)
	}

	userPrompt, err := g.userLoader.LoadTemplate(promptData)
	if err != nil {
		return "", pkgerrors.ErrLoadChecklistUserPromptFailed(err)
	}

	suffix := fmt.Sprintf("fix-iter%d", iteration)
	g.savePromptFile(params.TmpDir, params.SubjectID, promptIndex, suffix+"-system", systemPrompt)
	g.savePromptFile(params.TmpDir, params.SubjectID, promptIndex, suffix+"-user", userPrompt)

	mode := g.modeFactory.GetThinkMode()

	response, err := g.aiClient.ExecutePromptWithSystem(ctx, systemPrompt, userPrompt, "", mode)
	if err != nil {
		return "", pkgerrors.ErrChecklistAIEvaluationFailed(err)
	}

	g.savePromptFile(params.TmpDir, params.SubjectID, promptIndex, suffix+"-response", response)

	return response, nil
}

func (g *FixPromptGenerator) parseAndSaveResponse(response, resultPath string) (checklist.GenerateResult, error) {
	if g.hasQuestions(response) {
		questions, parseErr := g.parseQuestions(response)
		if parseErr != nil {
			slog.Warn("Failed to parse questions, treating as fix prompt", "error", parseErr)
		} else {
			slog.Info("AI needs clarification", "questionCount", len(questions))

			return checklist.GenerateResult{Questions: questions}, nil
		}
	}

	fixPrompt := g.extractFixPrompt(response, resultPath)
	if fixPrompt == "" {
		slog.Warn("No fix prompt content found in response")

		return checklist.GenerateResult{}, nil
	}

	err := os.WriteFile(resultPath, []byte(fixPrompt), fixPromptFilePermissions)
	if err != nil {
		slog.Warn("Failed to save fix prompt file", "path", resultPath, "error", err)
	} else {
		slog.Info("Fix prompt saved", "file", resultPath)
	}

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
	filePath := fmt.Sprintf("%s/%02d-%s-%s.txt", tmpDir, promptIndex, storyID, suffix)

	err := os.WriteFile(filePath, []byte(content), fixPromptFilePermissions)
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
