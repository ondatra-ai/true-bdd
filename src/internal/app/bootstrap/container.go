package bootstrap

import (
	"os"

	"github.com/ondatra-ai/true-bdd/src/adapters/ai"
	"github.com/ondatra-ai/true-bdd/src/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/src/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/src/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/src/internal/domain/ports"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/docs"
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
	models *provider.Registry,
	keys scenarioTripleConfigKeys,
) scenarioTriple {
	return scenarioTriple{
		evaluator: validate.NewChecklistEvaluatorWithPaths(
			aiClient, cfg, models,
			cfg.GetString(keys.checklistSystem),
			cfg.GetString(keys.checklist),
		),
		fixGenerator: validate.NewFixPromptGeneratorWithPaths(
			aiClient, cfg, models,
			cfg.GetString(keys.fixGeneratorSystem),
			cfg.GetString(keys.fixGenerator),
		),
		fixApplier: validate.NewFixApplierWithPaths(
			aiClient, cfg, models,
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
	DocResolver         *docs.Resolver
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

	configureLogging(cfg.GetString("paths.tmp_dir"))

	runDir, err := fs.NewRunDirectory(cfg.GetString("paths.tmp_dir"))
	if err != nil {
		return nil, pkgerrors.ErrCreateRunDirectoryFailed(err)
	}

	aiClient, models, err := newAIRouter(cfg, runDir)
	if err != nil {
		return nil, err
	}

	applyTrip := newScenarioTriple(aiClient, cfg, models, scenarioTripleConfigKeys{
		checklistSystem:    "templates.prompts.apply_checklist_system",
		checklist:          "templates.prompts.apply_checklist",
		fixGeneratorSystem: "templates.prompts.apply_fix_generator_system",
		fixGenerator:       "templates.prompts.apply_fix_generator",
		fixApplierSystem:   "templates.prompts.apply_fix_applier_system",
		fixApplier:         "templates.prompts.apply_fix_applier",
	})
	applyTrip.fixApplier.UseEditMode()

	buildTestsTrip := newScenarioTriple(aiClient, cfg, models, scenarioTripleConfigKeys{
		checklistSystem:    "templates.prompts.build_tests_checklist_system",
		checklist:          "templates.prompts.build_tests_checklist",
		fixGeneratorSystem: "templates.prompts.build_tests_fix_generator_system",
		fixGenerator:       "templates.prompts.build_tests_fix_generator",
		fixApplierSystem:   "templates.prompts.build_tests_fix_applier_system",
		fixApplier:         "templates.prompts.build_tests_fix_applier",
	})
	buildTestsTrip.fixApplier.UseEditMode()
	// build tests authors test files outside tmp. Unlike build code it
	// has no architecture.yaml to read roots from — its fixtures ship
	// none — so the roots are host config.
	buildTestsTrip.fixApplier.UseWriteRoots(testWriteGlobs(cfg))

	buildCodeTrip := newScenarioTriple(aiClient, cfg, models, scenarioTripleConfigKeys{
		checklistSystem:    "templates.prompts.build_code_checklist_system",
		checklist:          "templates.prompts.build_code_checklist",
		fixGeneratorSystem: "templates.prompts.build_code_fix_generator_system",
		fixGenerator:       "templates.prompts.build_code_fix_generator",
		fixApplierSystem:   "templates.prompts.build_code_fix_applier_system",
		fixApplier:         "templates.prompts.build_code_fix_applier",
	})
	buildCodeTrip.fixApplier.UseEditMode()

	testRunnerDispatcher := newTestRunnerDispatcher(runDir)

	return &Container{
		Config:                       cfg,
		RunDir:                       runDir,
		StoriesDir:                   cfg.GetString("paths.stories_dir"),
		EpicLoader:                   epic.NewEpicLoader(cfg),
		StoryLoader:                  story.NewStoryLoader(cfg),
		StoryScenarioParser:          story.NewStoryScenarioParser(cfg),
		ChecklistLoader:              checklist.NewChecklistLoader(cfg),
		DocResolver:                  docs.NewResolver(cfg),
		UserInputCollector:           input.NewUserInputCollector(),
		TableRenderer:                runner.NewTableRenderer(),
		Evaluator:                    validate.NewChecklistEvaluator(aiClient, cfg, models),
		FixGenerator:                 validate.NewFixPromptGenerator(aiClient, cfg, models),
		FixApplier:                   validate.NewFixApplier(aiClient, cfg, models),
		ApplyEvaluator:               applyTrip.evaluator,
		ApplyFixPromptGenerator:      applyTrip.fixGenerator,
		ApplyFixApplier:              applyTrip.fixApplier,
		RegistryLoader:               registry.NewRegistryLoader(),
		BuildTestsEvaluator:          buildTestsTrip.evaluator,
		BuildTestsFixPromptGenerator: buildTestsTrip.fixGenerator,
		BuildTestsFixApplier:         buildTestsTrip.fixApplier,
		ArchitectureLoader:           architecture.NewLoader(),
		TestRunnerDispatcher:         testRunnerDispatcher,
		BuildCodeEvaluator:           buildCodeTrip.evaluator,
		BuildCodeFixPromptGenerator:  buildCodeTrip.fixGenerator,
		BuildCodeFixApplier:          buildCodeTrip.fixApplier,
	}, nil
}

// newAIRouter resolves the model-tier table and builds the one
// provider router every checklist role shares. Which CLI actually runs
// a turn is decided per call by the tier the checklist selected, so a
// single router serves claude, crush, and codex alike.
func newAIRouter(cfg *config.ViperConfig, runDir *fs.RunDirectory) (*ai.Router, *provider.Registry, error) {
	models, err := NewModelRegistry(cfg)
	if err != nil {
		// A bad tier table is a CONFIG error, not an AI-client error;
		// saying so is the difference between a one-line fix and a hunt.
		return nil, nil, pkgerrors.ErrInvalidModelConfigFailed(err)
	}

	workDir, err := os.Getwd()
	if err != nil {
		return nil, nil, pkgerrors.ErrCreateAIClientFailed(err)
	}

	return ai.NewRouter(workDir, runDir.GetTmpOutPath()), models, nil
}

// testWriteGlobs resolves the roots `build tests --fix` may author
// into. The fallback matches the roots the build-tests fix-applier
// prompt names, so a host config that omits the key still gets an
// applier whose permissions agree with its instructions.
func testWriteGlobs(cfg *config.ViperConfig) []string {
	globs := cfg.GetStringSlice("paths.test_write_globs")
	if len(globs) > 0 {
		return globs
	}

	return []string{"./tests/**", "./services/**"}
}

// newTestRunnerDispatcher wires the three framework-specific runners
// behind the testrunner.Dispatcher used by `build code`. Each runner is
// constructed once; the dispatcher routes by the `framework:` field
// declared in architecture.yaml.
//
// All three share one Artifacts writer rooted at the run directory, so
// every framework's captured output lands beside the prompts the engine
// built from it and the sequence numbers order the whole run's test
// spawns against each other.
func newTestRunnerDispatcher(runDir *fs.RunDirectory) *testrunner.Dispatcher {
	artifacts := testrunner.NewArtifacts(runDir.GetTmpOutPath())

	return testrunner.NewDispatcher(map[string]testrunner.Runner{
		testrunner.FrameworkGoTest:     testrunner.NewGoTestRunner(artifacts),
		testrunner.FrameworkPlaywright: testrunner.NewPlaywrightRunner(artifacts),
		testrunner.FrameworkJest:       testrunner.NewJestRunner(artifacts),
	})
}
