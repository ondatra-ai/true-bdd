package validate

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"bdd-cli/src/adapters/ai"
	"bdd-cli/src/internal/domain/ports"
	"bdd-cli/src/internal/infrastructure/config"
	"bdd-cli/src/internal/infrastructure/template"
	pkgerrors "bdd-cli/src/internal/pkg/errors"
)

const fixApplierFilePermissions = 0o644

// FixApplierData represents data needed for fix applier templates.
type FixApplierData struct {
	Subject    any    // Current subject to modify (e.g., *story.Story or *ScenarioApplyData)
	SubjectID  string // Subject identifier
	FixPrompt  string // The fix prompt to apply
	ResultPath string // Path for FILE_START/FILE_END output
}

// FixApplier applies fix prompts to stories using AI.
type FixApplier struct {
	aiClient     ports.AIPort
	config       *config.ViperConfig
	modeFactory  *ai.ModeFactory
	systemLoader *template.TemplateLoader[FixApplierData]
	userLoader   *template.TemplateLoader[FixApplierData]
	tmpDir       string
}

// NewFixApplier creates a new fix applier with config-based template paths.
func NewFixApplier(aiClient ports.AIPort, cfg *config.ViperConfig) *FixApplier {
	systemTemplatePath := cfg.GetString("templates.prompts.fix_applier_system")
	userTemplatePath := cfg.GetString("templates.prompts.fix_applier")

	return NewFixApplierWithPaths(aiClient, cfg, systemTemplatePath, userTemplatePath)
}

// NewFixApplierWithPaths creates a new fix applier with explicit template paths.
func NewFixApplierWithPaths(
	aiClient ports.AIPort,
	cfg *config.ViperConfig,
	systemPath, userPath string,
) *FixApplier {
	return &FixApplier{
		aiClient:     aiClient,
		config:       cfg,
		modeFactory:  ai.NewModeFactory(cfg),
		systemLoader: template.NewTemplateLoader[FixApplierData](systemPath),
		userLoader:   template.NewTemplateLoader[FixApplierData](userPath),
	}
}

// Apply applies a fix prompt to the subject and returns the extracted content as a string.
// The caller is responsible for parsing the returned content into the appropriate type.
func (a *FixApplier) Apply(
	ctx context.Context,
	subject any,
	subjectID string,
	fixPrompt string,
	tmpDir string,
	iteration int,
) (string, error) {
	a.tmpDir = tmpDir

	slog.Info("Applying fix prompt",
		"subjectID", subjectID,
		"iteration", iteration,
	)

	resultPath := fmt.Sprintf("%s/apply-%s-iter%d-result.yaml", tmpDir, subjectID, iteration)

	promptData := FixApplierData{
		Subject:    subject,
		SubjectID:  subjectID,
		FixPrompt:  fixPrompt,
		ResultPath: resultPath,
	}

	systemPrompt, err := a.systemLoader.LoadTemplate(promptData)
	if err != nil {
		return "", pkgerrors.ErrLoadChecklistSystemPromptFailed(err)
	}

	userPrompt, err := a.userLoader.LoadTemplate(promptData)
	if err != nil {
		return "", pkgerrors.ErrLoadChecklistUserPromptFailed(err)
	}

	// Save prompts for debugging
	a.savePromptFile(subjectID, iteration, "system", systemPrompt)
	a.savePromptFile(subjectID, iteration, "user", userPrompt)

	mode := a.modeFactory.GetThinkMode()

	response, err := a.aiClient.ExecutePromptWithSystem(ctx, systemPrompt, userPrompt, "", mode)
	if err != nil {
		return "", pkgerrors.ErrChecklistAIEvaluationFailed(err)
	}

	// Save response for debugging
	a.savePromptFile(subjectID, iteration, "response", response)

	// Extract content from response
	content := ExtractFileContent(response, resultPath)
	if content == "" {
		return "", pkgerrors.ErrFixApplierNoContentFound(resultPath)
	}

	// Save the extracted content
	writeErr := os.WriteFile(resultPath, []byte(content), fixApplierFilePermissions)
	if writeErr != nil {
		slog.Warn("Failed to save result file", "path", resultPath, "error", writeErr)
	}

	slog.Info("Fix applied successfully", "subjectID", subjectID)

	return content, nil
}

// savePromptFile saves a prompt file for debugging.
func (a *FixApplier) savePromptFile(storyID string, iteration int, suffix, content string) {
	if a.tmpDir == "" {
		return
	}

	filePath := fmt.Sprintf("%s/apply-%s-iter%d-%s.txt", a.tmpDir, storyID, iteration, suffix)

	err := os.WriteFile(filePath, []byte(content), fixApplierFilePermissions)
	if err != nil {
		slog.Warn("Failed to save prompt file", "error", err)
	} else {
		slog.Info("Prompt saved", "file", filePath)
	}
}
