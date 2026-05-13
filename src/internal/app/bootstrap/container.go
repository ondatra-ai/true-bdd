package bootstrap

import (
	"bdd-cli/src/adapters/ai"
	"bdd-cli/src/internal/app/commands"
	"bdd-cli/src/internal/app/generators/validate"
	"bdd-cli/src/internal/domain/ports"
	"bdd-cli/src/internal/infrastructure/checklist"
	"bdd-cli/src/internal/infrastructure/config"
	"bdd-cli/src/internal/infrastructure/epic"
	"bdd-cli/src/internal/infrastructure/fs"
	"bdd-cli/src/internal/infrastructure/input"
	"bdd-cli/src/internal/infrastructure/story"
	pkgerrors "bdd-cli/src/internal/pkg/errors"
)

// scenarioTriple bundles the (evaluator, fix-prompt-generator,
// fix-applier) trio that every scenario-walking command depends on.
// Used internally by NewContainer to keep the bootstrap function under
// the configured complexity / length budgets.
type scenarioTriple struct {
	evaluator    *validate.ChecklistEvaluator
	fixGenerator *validate.FixPromptGenerator
	fixApplier   *validate.FixApplier
}

// scenarioTripleConfigKeys names the bdd-cli.yaml config paths for one
// scenario-walking command's evaluator / fix-generator / fix-applier
// templates.
type scenarioTripleConfigKeys struct {
	checklistSystem    string
	checklist          string
	fixGeneratorSystem string
	fixGenerator       string
	fixApplierSystem   string
	fixApplier         string
}

func newScenarioTriple(
	aiClient ports.AIPort,
	cfg *config.ViperConfig,
	keys scenarioTripleConfigKeys,
) scenarioTriple {
	return scenarioTriple{
		evaluator: validate.NewChecklistEvaluatorWithPaths(
			aiClient, cfg,
			cfg.GetString(keys.checklistSystem),
			cfg.GetString(keys.checklist),
		),
		fixGenerator: validate.NewFixPromptGeneratorWithPaths(
			aiClient, cfg,
			cfg.GetString(keys.fixGeneratorSystem),
			cfg.GetString(keys.fixGenerator),
		),
		fixApplier: validate.NewFixApplierWithPaths(
			aiClient, cfg,
			cfg.GetString(keys.fixApplierSystem),
			cfg.GetString(keys.fixApplier),
		),
	}
}

// Container wires together the components needed by the CLI.
type Container struct {
	Config          *config.ViperConfig
	USValidationCmd *commands.USValidationCommand
	// Apply-flavored evaluator / fix-prompt / fix-applier triple driving
	// `us apply`. The parser reads a refined story file
	// (acceptance_criteria[].steps shape) and emits one ScenarioApplyData
	// per AC; the triple uses the templates.prompts.apply_* templates and
	// targets the scratch copy of docs/requirements.yaml.
	StoryScenarioParser     *story.StoryScenarioParser
	ApplyEvaluator          *validate.ChecklistEvaluator
	ApplyFixPromptGenerator *validate.FixPromptGenerator
	ApplyFixApplier         *validate.FixApplier
	RunDir                  *fs.RunDirectory
}

// NewContainer builds the Container.
func NewContainer() (*Container, error) {
	cfg, err := config.NewViperConfig()
	if err != nil {
		return nil, pkgerrors.ErrInitializeConfigFailed(err)
	}

	configureLogging()

	runDir, err := fs.NewRunDirectory(cfg.GetString("paths.tmp_dir"))
	if err != nil {
		return nil, pkgerrors.ErrCreateRunDirectoryFailed(err)
	}

	epicLoader := epic.NewEpicLoader(cfg)

	claudeClient, err := ai.NewClaudeClient()
	if err != nil {
		return nil, pkgerrors.ErrCreateAIClientFailed(err)
	}

	storyLoader := story.NewStoryLoader(cfg)
	userInputCollector := input.NewUserInputCollector()

	checklistLoader := checklist.NewChecklistLoader(cfg)

	evaluator := validate.NewChecklistEvaluator(claudeClient, cfg)
	fixPromptGenerator := validate.NewFixPromptGenerator(claudeClient, cfg)
	fixApplier := validate.NewFixApplier(claudeClient, cfg)

	tableRenderer := commands.NewTableRenderer()
	storiesDir := cfg.GetString("paths.stories_dir")

	usValidationCmd := commands.NewUSValidationCommand(
		epicLoader,
		storyLoader,
		checklistLoader,
		evaluator,
		fixPromptGenerator,
		fixApplier,
		userInputCollector,
		tableRenderer,
		runDir,
		storiesDir,
	)

	// Apply-flavored evaluator / fix-prompt / fix-applier set used by
	// `us apply`. Templates live under templates.prompts.apply_* and
	// are written for the merge-into-requirements.yaml subject.
	applyTrip := newScenarioTriple(claudeClient, cfg, scenarioTripleConfigKeys{
		checklistSystem:    "templates.prompts.apply_checklist_system",
		checklist:          "templates.prompts.apply_checklist",
		fixGeneratorSystem: "templates.prompts.apply_fix_generator_system",
		fixGenerator:       "templates.prompts.apply_fix_generator",
		fixApplierSystem:   "templates.prompts.apply_fix_applier_system",
		fixApplier:         "templates.prompts.apply_fix_applier",
	})

	storyScenarioParser := story.NewStoryScenarioParser(cfg)

	return &Container{
		Config:                  cfg,
		USValidationCmd:         usValidationCmd,
		StoryScenarioParser:     storyScenarioParser,
		ApplyEvaluator:          applyTrip.evaluator,
		ApplyFixPromptGenerator: applyTrip.fixGenerator,
		ApplyFixApplier:         applyTrip.fixApplier,
		RunDir:                  runDir,
	}, nil
}
