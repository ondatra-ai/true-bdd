package validate

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/ondatra-ai/true-bdd/src/adapters/ai"
	"github.com/ondatra-ai/true-bdd/src/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/src/internal/domain/ports"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/template"
	pkgerrors "github.com/ondatra-ai/true-bdd/src/internal/pkg/errors"
)

const fixApplierFilePermissions = 0o644

// FixApplierData represents data needed for fix applier templates.
type FixApplierData struct {
	Subject    any    // Current subject to modify (e.g., *story.Story or *ScenarioApplyData)
	SubjectID  string // Subject identifier
	FixPrompt  string // The fix prompt to apply
	ResultPath string // Path for FILE_START/FILE_END output
}

// ApplyParams contains the parameters for one fix application.
type ApplyParams struct {
	Subject   any    // Current subject to modify
	SubjectID string // Subject identifier
	FixPrompt string // The concrete fix prompt to apply
	TmpDir    string // Run artifact directory
	Iteration int    // 1-based apply counter, for file naming
	// ModelTier names the tier that writes the fix. Resolved by the
	// evaluator while the prompt was in scope and carried here on the
	// failed check; empty means engine.default_model.
	ModelTier string
}

// FixApplier applies fix prompts to stories using AI.
type FixApplier struct {
	aiClient     ports.AIPort
	config       *config.ViperConfig
	models       *provider.Registry
	modeFactory  *ai.ModeFactory
	systemLoader *template.TemplateLoader[FixApplierData]
	userLoader   *template.TemplateLoader[FixApplierData]
	tmpDir       string
	// useEditMode toggles the Claude tool-permission set used by
	// Apply. Default false → ThinkMode (Edit disallowed) for handlers
	// that emit FILE_START/FILE_END markers. Set true via
	// UseEditMode() for handlers that mutate the scratch file in
	// place via the Edit tool (us apply's F: prompts).
	useEditMode bool
}

// NewFixApplier creates a new fix applier with config-based template paths.
func NewFixApplier(
	aiClient ports.AIPort,
	cfg *config.ViperConfig,
	models *provider.Registry,
) *FixApplier {
	systemTemplatePath := cfg.GetString("templates.prompts.fix_applier_system")
	userTemplatePath := cfg.GetString("templates.prompts.fix_applier")

	return NewFixApplierWithPaths(aiClient, cfg, models, systemTemplatePath, userTemplatePath)
}

// NewFixApplierWithPaths creates a new fix applier with explicit template paths.
func NewFixApplierWithPaths(
	aiClient ports.AIPort,
	cfg *config.ViperConfig,
	models *provider.Registry,
	systemPath, userPath string,
) *FixApplier {
	return &FixApplier{
		aiClient:     aiClient,
		config:       cfg,
		models:       models,
		modeFactory:  ai.NewModeFactory(cfg),
		systemLoader: template.NewTemplateLoader[FixApplierData](systemPath),
		userLoader:   template.NewTemplateLoader[FixApplierData](userPath),
	}
}

// UseEditMode configures this applier to allow the Edit and
// MultiEdit tools against the scratch path on each Apply call.
// Required for the us-apply F: handlers, whose prompts instruct
// Claude to edit the scratch registry directly.
func (a *FixApplier) UseEditMode() {
	a.useEditMode = true
}

// Apply applies a fix prompt to the subject and returns the extracted content as a string.
// The caller is responsible for parsing the returned content into the appropriate type.
func (a *FixApplier) Apply(ctx context.Context, params ApplyParams) (string, error) {
	a.tmpDir = params.TmpDir

	slog.Info("Applying fix prompt",
		"subjectID", params.SubjectID,
		"iteration", params.Iteration,
	)

	resultPath := fmt.Sprintf("%s/apply-%s-iter%d-result.yaml",
		params.TmpDir, sanitizeID(params.SubjectID), params.Iteration)

	promptData := FixApplierData{
		Subject:    params.Subject,
		SubjectID:  params.SubjectID,
		FixPrompt:  params.FixPrompt,
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
	a.savePromptFile(params.SubjectID, params.Iteration, "system", systemPrompt)
	a.savePromptFile(params.SubjectID, params.Iteration, "user", userPrompt)

	mode := a.selectMode()

	// This is the turn that writes files, so it is the one a checklist
	// typically points at the `coder` tier.
	model, err := a.models.ResolveName(params.ModelTier)
	if err != nil {
		return "", pkgerrors.ErrResolveModelTierFailed("fix application", err)
	}

	response, err := a.aiClient.ExecutePromptWithSystem(ctx, systemPrompt, userPrompt, model, mode)
	if err != nil {
		return "", pkgerrors.ErrChecklistAIEvaluationFailed(err)
	}

	// Save response for debugging
	a.savePromptFile(params.SubjectID, params.Iteration, "response", response)

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

	slog.Info("Fix applied successfully", "subjectID", params.SubjectID)

	return content, nil
}

// selectMode returns the Claude execution mode for this Apply call —
// EditMode (Edit/MultiEdit allowed on scratch) when the applier was
// configured via UseEditMode(), ThinkMode (Edit disallowed) otherwise.
func (a *FixApplier) selectMode() ai.ExecutionMode {
	if a.useEditMode {
		return a.modeFactory.GetEditMode()
	}

	return a.modeFactory.GetThinkMode()
}

// savePromptFile saves a prompt file for debugging.
func (a *FixApplier) savePromptFile(storyID string, iteration int, suffix, content string) {
	if a.tmpDir == "" {
		return
	}

	filePath := fmt.Sprintf("%s/apply-%s-iter%d-%s.txt", a.tmpDir, sanitizeID(storyID), iteration, suffix)

	err := os.WriteFile(filePath, []byte(content), fixApplierFilePermissions)
	if err != nil {
		slog.Warn("Failed to save prompt file", "error", err)
	} else {
		slog.Info("Prompt saved", "file", filePath)
	}
}
