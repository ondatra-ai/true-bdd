package validate

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/ports"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/template"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

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
	// failed check; empty means engine.default_apply_model.
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
	// useEditMode toggles Apply's tool-permission set: false (default) is
	// ThinkMode for FILE_START/FILE_END handlers; true, set via
	// UseEditMode(), is EditMode for handlers that mutate the scratch file directly (us apply's F: prompts).
	useEditMode bool
	// writeRoots are project trees outside tmp this applier may write,
	// for the `build` commands whose whole job is authoring source. Set
	// via UseWriteRoots(); empty keeps the tmp-only behaviour.
	writeRoots []string
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

// UseEditMode configures this applier to allow the Edit and MultiEdit tools
// against the scratch path on each Apply call — required for the us-apply
// F: handlers, whose prompts instruct Claude to edit the scratch registry directly.
func (a *FixApplier) UseEditMode() {
	a.useEditMode = true
}

// UseWriteRoots grants this applier write access to project trees outside
// tmp — the `services/*` roots for build code, test roots for build tests.
// Without it, ExecutionMode (crush guard, codex sandbox) denies every write the system prompt instructs.
func (a *FixApplier) UseWriteRoots(roots []string) {
	a.writeRoots = roots
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
	model, err := a.models.ResolveRole(provider.RoleApply, params.ModelTier)
	if err != nil {
		return "", pkgerrors.ErrResolveModelTierFailed("fix application", err)
	}

	response, err := a.aiClient.ExecutePromptWithSystem(
		// No result schema: this repo routes every apply turn to crush,
		// which cannot enforce one (adapters/ai.SupportsResultSchema).
		ctx, provider.RoleApply, systemPrompt, userPrompt, model, mode, "",
	)
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
	writeErr := disk.Write(resultPath, []byte(content), disk.Shared)
	if writeErr != nil {
		slog.Warn("Failed to save result file", "path", resultPath, "error", writeErr)
	}

	// A turn that ran to completion is not the same as a fix that landed: the
	// applier can report `applied: false` when the write it was told to make is
	// impossible — treating that as success let a blocked applier spin until an external timeout.
	appliedErr := checkFixApplied(content)
	if appliedErr != nil {
		slog.Warn("Fix was NOT applied",
			"subjectID", params.SubjectID,
			"iteration", params.Iteration,
			"error", appliedErr,
		)

		return "", appliedErr
	}

	slog.Info("Fix applied successfully", "subjectID", params.SubjectID)

	return content, nil
}

// applyResultYAML is the confirmation block the fix-applier templates ask
// for. Applied is a POINTER: nil (no `applied:` key, as us create/refine's
// story body emits) must stay distinguishable from explicit false, or every such turn misreports as a failed fix.
type applyResultYAML struct {
	Applied *bool  `yaml:"applied"`
	Target  string `yaml:"target"`
	Summary string `yaml:"summary"`
}

// checkFixApplied returns an error when the applier explicitly reported
// that it wrote nothing. Content it cannot parse is passed through: several
// appliers return a document body rather than a confirmation block.
func checkFixApplied(content string) error {
	result, ok := parseApplyResult(content)
	if !ok {
		// The block did not decode — before concluding "not a confirmation block",
		// scan for a literal `applied:` line: an unquoted `": "` in `target:` makes
		// invalid YAML, and reading that as pass-through would turn a refusal into "Fix applied successfully".
		return checkFixAppliedFallback(content)
	}

	// Three ways to have no objection: the content is not a
	// confirmation block at all, the block omits `applied:`, or it
	// reports success. Only an explicit false is a refusal to act on.
	if result.Applied == nil || *result.Applied {
		return nil
	}

	return pkgerrors.ErrFixNotAppliedByModel(result.Target, result.Summary)
}

// appliedLinePattern matches the `applied:` key of a confirmation block
// in content YAML could not decode.
var appliedLinePattern = regexp.MustCompile(`(?mi)^\s*applied:\s*(true|false)\s*$`)

// checkFixAppliedFallback salvages the verdict from unparseable content.
// Only an explicit `applied: false` is actionable. Every status line is
// scanned, not just the first, so a later refusal can never be masked by an earlier success.
func checkFixAppliedFallback(content string) error {
	matches := appliedLinePattern.FindAllStringSubmatch(stripMarkdownFences(content), -1)
	if len(matches) == 0 {
		return nil
	}

	for _, match := range matches {
		if strings.EqualFold(match[1], "false") {
			slog.Warn("Fix applier confirmation block is not valid YAML; " +
				"read applied: false from the raw text")

			return pkgerrors.ErrFixNotAppliedByModel("", "")
		}
	}

	slog.Warn("Fix applier confirmation block is not valid YAML; " +
		"read applied: true from the raw text")

	return nil
}

// parseApplyResult decodes an applier confirmation block. Content that is
// not one — a story body from us create/refine, or prose — reports ok=false
// rather than an error: not being a confirmation block is normal here.
func parseApplyResult(content string) (applyResultYAML, bool) {
	var result applyResultYAML

	if yaml.Unmarshal([]byte(stripMarkdownFences(content)), &result) != nil {
		return applyResultYAML{}, false
	}

	return result, true
}

// selectMode returns the execution mode for this Apply call: SourceEditMode
// when write roots were declared, EditMode (Edit/MultiEdit on scratch) when
// configured via UseEditMode(), ThinkMode (Edit disallowed) otherwise.
func (a *FixApplier) selectMode() ai.ExecutionMode {
	if len(a.writeRoots) > 0 {
		return a.modeFactory.GetSourceEditMode(a.writeRoots)
	}

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

	err := disk.Write(filePath, []byte(content), disk.Shared)
	if err != nil {
		slog.Warn("Failed to save prompt file", "error", err)
	} else {
		slog.Info("Prompt saved", "file", filePath)
	}
}
