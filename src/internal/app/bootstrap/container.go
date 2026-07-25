package bootstrap

import (
	"github.com/ondatra-ai/true-bdd/src/adapters/ai"
	"github.com/ondatra-ai/true-bdd/src/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/src/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/src/internal/domain/ports"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/epic"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/fs"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/input"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/story"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/testrunner"
	pkgerrors "github.com/ondatra-ai/true-bdd/src/internal/pkg/errors"
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

// scenarioTripleConfigKeys names the true-bdd.yaml config paths for one
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

// Container wires together the components needed by the CLI. Per-command
// deps (CreateDeps, RefineDeps, ApplyDeps) are projected from this on
// the cobra side so each command depends on only what it actually uses.
type Container struct {
	Config              *config.ViperConfig
	RunDir              *fs.RunDirectory
	StoriesDir          string
	EpicLoader          *epic.EpicLoader
	StoryLoader         *story.StoryLoader
	StoryScenarioParser *story.StoryScenarioParser
	ChecklistLoader     *checklist.ChecklistLoader
	UserInputCollector  *input.UserInputCollector
	TableRenderer       *runner.TableRenderer
	// Standard generator triple drives `us create` and `us refine`.
	Evaluator    *validate.ChecklistEvaluator
	FixGenerator *validate.FixPromptGenerator
	FixApplier   *validate.FixApplier
	// Apply-flavored triple drives `us apply`. Templates live under
	// templates.prompts.apply_* and the fix-applier is configured with
	// EditMode so Claude can mutate the scratch registry directly.
	ApplyEvaluator          *validate.ChecklistEvaluator
	ApplyFixPromptGenerator *validate.FixPromptGenerator
	ApplyFixApplier         *validate.FixApplier
	// Build-tests triple drives `build tests`. Templates live under
	// templates.prompts.build_tests_* and the fix-applier runs in
	// EditMode so Claude can Write/Edit test files in place.
	RegistryLoader               *registry.RegistryLoader
	BuildTestsEvaluator          *validate.ChecklistEvaluator
	BuildTestsFixPromptGenerator *validate.FixPromptGenerator
	BuildTestsFixApplier         *validate.FixApplier
	// Build-code triple drives `build code`. Templates live under
	// templates.prompts.build_code_* and the fix-applier runs in
	// EditMode so Claude can Write/Edit production source files under
	// services/* in place. The dispatcher routes test discovery and
	// per-test reruns to one of the framework-specific runners.
	ArchitectureLoader          *architecture.Loader
	TestRunnerDispatcher        *testrunner.Dispatcher
	BuildCodeEvaluator          *validate.ChecklistEvaluator
	BuildCodeFixPromptGenerator *validate.FixPromptGenerator
	BuildCodeFixApplier         *validate.FixApplier
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

	claudeClient, err := ai.NewClaudeClient()
	if err != nil {
		return nil, pkgerrors.ErrCreateAIClientFailed(err)
	}

	applyTrip := newScenarioTriple(claudeClient, cfg, scenarioTripleConfigKeys{
		checklistSystem:    "templates.prompts.apply_checklist_system",
		checklist:          "templates.prompts.apply_checklist",
		fixGeneratorSystem: "templates.prompts.apply_fix_generator_system",
		fixGenerator:       "templates.prompts.apply_fix_generator",
		fixApplierSystem:   "templates.prompts.apply_fix_applier_system",
		fixApplier:         "templates.prompts.apply_fix_applier",
	})
	applyTrip.fixApplier.UseEditMode()

	buildTestsTrip := newScenarioTriple(claudeClient, cfg, scenarioTripleConfigKeys{
		checklistSystem:    "templates.prompts.build_tests_checklist_system",
		checklist:          "templates.prompts.build_tests_checklist",
		fixGeneratorSystem: "templates.prompts.build_tests_fix_generator_system",
		fixGenerator:       "templates.prompts.build_tests_fix_generator",
		fixApplierSystem:   "templates.prompts.build_tests_fix_applier_system",
		fixApplier:         "templates.prompts.build_tests_fix_applier",
	})
	buildTestsTrip.fixApplier.UseEditMode()

	buildCodeTrip := newScenarioTriple(claudeClient, cfg, scenarioTripleConfigKeys{
		checklistSystem:    "templates.prompts.build_code_checklist_system",
		checklist:          "templates.prompts.build_code_checklist",
		fixGeneratorSystem: "templates.prompts.build_code_fix_generator_system",
		fixGenerator:       "templates.prompts.build_code_fix_generator",
		fixApplierSystem:   "templates.prompts.build_code_fix_applier_system",
		fixApplier:         "templates.prompts.build_code_fix_applier",
	})
	buildCodeTrip.fixApplier.UseEditMode()

	testRunnerDispatcher := newTestRunnerDispatcher()

	return &Container{
		Config:                       cfg,
		RunDir:                       runDir,
		StoriesDir:                   cfg.GetString("paths.stories_dir"),
		EpicLoader:                   epic.NewEpicLoader(cfg),
		StoryLoader:                  story.NewStoryLoader(cfg),
		StoryScenarioParser:          story.NewStoryScenarioParser(cfg),
		ChecklistLoader:              checklist.NewChecklistLoader(cfg),
		UserInputCollector:           input.NewUserInputCollector(),
		TableRenderer:                runner.NewTableRenderer(),
		Evaluator:                    validate.NewChecklistEvaluator(claudeClient, cfg),
		FixGenerator:                 validate.NewFixPromptGenerator(claudeClient, cfg),
		FixApplier:                   validate.NewFixApplier(claudeClient, cfg),
		ApplyEvaluator:               applyTrip.evaluator,
		ApplyFixPromptGenerator:      applyTrip.fixGenerator,
		ApplyFixApplier:              applyTrip.fixApplier,
		RegistryLoader:               registry.NewRegistryLoader(),
		BuildTestsEvaluator:          buildTestsTrip.evaluator,
		BuildTestsFixPromptGenerator: buildTestsTrip.fixGenerator,
		BuildTestsFixApplier:         buildTestsTrip.fixApplier,
		ArchitectureLoader:           architecture.NewLoader(cfg),
		TestRunnerDispatcher:         testRunnerDispatcher,
		BuildCodeEvaluator:           buildCodeTrip.evaluator,
		BuildCodeFixPromptGenerator:  buildCodeTrip.fixGenerator,
		BuildCodeFixApplier:          buildCodeTrip.fixApplier,
	}, nil
}

// newTestRunnerDispatcher wires the three framework-specific runners
// behind the testrunner.Dispatcher used by `build code`. Each runner is
// stateless and constructed once; the dispatcher routes by the
// `framework:` field declared in architecture.yaml.
func newTestRunnerDispatcher() *testrunner.Dispatcher {
	return testrunner.NewDispatcher(map[string]testrunner.Runner{
		testrunner.FrameworkGoTest:     testrunner.NewGoTestRunner(),
		testrunner.FrameworkPlaywright: testrunner.NewPlaywrightRunner(),
		testrunner.FrameworkJest:       testrunner.NewJestRunner(),
	})
}
