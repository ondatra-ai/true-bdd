package bootstrap

import (
	"os"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/adapters/ai"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/ports"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/fs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/input"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/story"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

// scenarioTriple bundles the (evaluator, fix-prompt-generator,
// fix-applier) trio that every scenario-walking command depends on,
// keeping NewContainer under the configured complexity budget.
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
// deps (RefineDeps, ApplyDeps) are projected from this on
// the cobra side so each command depends on only what it actually uses.
type Container struct {
	Config              *config.ViperConfig
	RunDir              *fs.RunDirectory
	StoriesDir          string
	StoryLoader         *story.StoryLoader
	StoryScenarioParser *story.StoryScenarioParser
	ChecklistLoader     *checklist.ChecklistLoader
	DocResolver         *docs.Resolver
	UserInputCollector  *input.UserInputCollector
	TableRenderer       *runner.TableRenderer
	// Refine triple drives `us refine`, the only command on the standard
	// templates; its fix-applier runs in EditMode for the term documents.
	RefineEvaluator          *validate.ChecklistEvaluator
	RefineFixPromptGenerator *validate.FixPromptGenerator
	RefineFixApplier         *validate.FixApplier
	// Apply-flavored triple drives `us apply`. Templates live under
	// templates.prompts.apply_* and the fix-applier is configured with
	// EditMode so Claude can mutate the scratch registry directly.
	ApplyEvaluator               *validate.ChecklistEvaluator
	ApplyFixPromptGenerator      *validate.FixPromptGenerator
	ApplyFixApplier              *validate.FixApplier
	RegistryLoader               *registry.RegistryLoader
	BuildStepsCritic             *validate.StepCritic
	BuildStepsEvaluator          *validate.ChecklistEvaluator
	BuildStepsFixPromptGenerator *validate.FixPromptGenerator
	BuildStepsFixApplier         *validate.FixApplier
	// Build-code triple drives `build code`: templates under
	// templates.prompts.build_code_*, fix-applier in EditMode writing
	// services/* files. The dispatcher below routes per-framework runs.
	TestRunnerDispatcher        *testrunner.Dispatcher
	BuildCodeEvaluator          *validate.ChecklistEvaluator
	BuildCodeFixPromptGenerator *validate.FixPromptGenerator
	BuildCodeFixApplier         *validate.FixApplier
	// Scen-check triple drives `scen check`. Templates live under
	// templates.prompts.scen_check_*; every shipped prompt carries an F.
	ScenCheckEvaluator          *validate.ChecklistEvaluator
	ScenCheckFixPromptGenerator *validate.FixPromptGenerator
	ScenCheckFixApplier         *validate.FixApplier
}

// NewContainer builds the Container.
func NewContainer(overrides ...string) (*Container, error) {
	cfg, err := config.NewViperConfig()
	if err != nil {
		return nil, pkgerrors.ErrInitializeConfigFailed(err)
	}

	configureLogging(cfg.GetString("paths.tmp_dir"))

	runDir, err := fs.NewRunDirectory(cfg.GetString("paths.tmp_dir"))
	if err != nil {
		return nil, pkgerrors.ErrCreateRunDirectoryFailed(err)
	}

	aiClient, models, err := newAIRouter(cfg, runDir, overrides...)
	if err != nil {
		return nil, err
	}

	err = product.ValidateForm(cfg.GetString("documents.product"))
	if err != nil {
		return nil, err
	}

	trips := newTriples(aiClient, cfg, models)

	testRunnerDispatcher := newTestRunnerDispatcher(runDir)

	// EditMode reaches tmp only: a closure fix edits the scratch copy of the
	// product document; a model Prompt's fix still returns the Story body.
	refineFixApplier := validate.NewFixApplier(aiClient, cfg, models)
	refineFixApplier.UseEditMode()

	return &Container{
		Config:                   cfg,
		RunDir:                   runDir,
		StoriesDir:               cfg.GetString("paths.stories_dir"),
		StoryLoader:              story.NewStoryLoader(cfg),
		StoryScenarioParser:      story.NewStoryScenarioParser(cfg),
		ChecklistLoader:          checklist.NewChecklistLoader(cfg),
		DocResolver:              docs.NewResolver(cfg),
		UserInputCollector:       input.NewUserInputCollector(),
		TableRenderer:            runner.NewTableRenderer(),
		RefineEvaluator:          validate.NewChecklistEvaluator(aiClient, cfg, models),
		RefineFixPromptGenerator: validate.NewFixPromptGenerator(aiClient, cfg, models),
		RefineFixApplier:         refineFixApplier,
		ApplyEvaluator:           trips.apply.evaluator,
		ApplyFixPromptGenerator:  trips.apply.fixGenerator,
		ApplyFixApplier:          trips.apply.fixApplier,
		RegistryLoader:           registry.NewRegistryLoader(),
		BuildStepsCritic: validate.NewStepCritic(aiClient, models,
			cfg.GetString("templates.prompts.build_steps_critique_system"),
			cfg.GetString("templates.prompts.build_steps_critique")),
		BuildStepsEvaluator:          trips.buildSteps.evaluator,
		BuildStepsFixPromptGenerator: trips.buildSteps.fixGenerator,
		BuildStepsFixApplier:         trips.buildSteps.fixApplier,
		TestRunnerDispatcher:         testRunnerDispatcher,
		BuildCodeEvaluator:           trips.buildCode.evaluator,
		BuildCodeFixPromptGenerator:  trips.buildCode.fixGenerator,
		BuildCodeFixApplier:          trips.buildCode.fixApplier,
		ScenCheckEvaluator:           trips.scenCheck.evaluator,
		ScenCheckFixPromptGenerator:  trips.scenCheck.fixGenerator,
		ScenCheckFixApplier:          trips.scenCheck.fixApplier,
	}, nil
}

// newAIRouter resolves the model-tier table and builds the one
// provider router every checklist role shares — which CLI runs a turn
// is decided per call by the tier, not by which router served it.
func newAIRouter(
	cfg *config.ViperConfig, runDir *fs.RunDirectory, overrides ...string,
) (*ai.Router, *provider.Registry, error) {
	models, err := NewModelRegistry(cfg, overrides...)
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

// newTestRunnerDispatcher wires the three framework-specific runners
// behind the `build code` Dispatcher, routed by architecture.yaml's
// `framework:` field. All three share one Artifacts writer.
func newTestRunnerDispatcher(runDir *fs.RunDirectory) *testrunner.Dispatcher {
	artifacts := testrunner.NewArtifacts(runDir.GetTmpOutPath())

	return testrunner.NewDispatcher(map[string]testrunner.Runner{
		testrunner.FrameworkGoTest:     testrunner.NewGoTestRunner(artifacts),
		testrunner.FrameworkPlaywright: testrunner.NewPlaywrightRunner(artifacts),
		testrunner.FrameworkJest:       testrunner.NewJestRunner(artifacts),
	})
}

// triples groups the four command-flavoured generator triples, so
// NewContainer stays a wiring list rather than a construction site.
type triples struct {
	apply      scenarioTriple
	buildCode  scenarioTriple
	buildSteps scenarioTriple
	scenCheck  scenarioTriple
}

// newTriples builds each command's generator triple and applies the
// per-command applier mode. Which appliers may write, and where, is
// decided here and nowhere else.
func newTriples(
	aiClient ports.AIPort,
	cfg *config.ViperConfig,
	models *provider.Registry,
) triples {
	applyTrip := newScenarioTriple(aiClient, cfg, models, scenarioTripleConfigKeys{
		checklistSystem:    "templates.prompts.apply_checklist_system",
		checklist:          "templates.prompts.apply_checklist",
		fixGeneratorSystem: "templates.prompts.apply_fix_generator_system",
		fixGenerator:       "templates.prompts.apply_fix_generator",
		fixApplierSystem:   "templates.prompts.apply_fix_applier_system",
		fixApplier:         "templates.prompts.apply_fix_applier",
	})
	applyTrip.fixApplier.UseEditMode()

	buildCodeTrip := newScenarioTriple(aiClient, cfg, models, scenarioTripleConfigKeys{
		checklistSystem:    "templates.prompts.build_code_checklist_system",
		checklist:          "templates.prompts.build_code_checklist",
		fixGeneratorSystem: "templates.prompts.build_code_fix_generator_system",
		fixGenerator:       "templates.prompts.build_code_fix_generator",
		fixApplierSystem:   "templates.prompts.build_code_fix_applier_system",
		fixApplier:         "templates.prompts.build_code_fix_applier",
	})
	buildCodeTrip.fixApplier.UseEditMode()

	scenCheckTrip := newScenarioTriple(aiClient, cfg, models, scenarioTripleConfigKeys{
		checklistSystem:    "templates.prompts.scen_check_checklist_system",
		checklist:          "templates.prompts.scen_check_checklist",
		fixGeneratorSystem: "templates.prompts.scen_check_fix_generator_system",
		fixGenerator:       "templates.prompts.scen_check_fix_generator",
		fixApplierSystem:   "templates.prompts.scen_check_fix_applier_system",
		fixApplier:         "templates.prompts.scen_check_fix_applier",
	})
	// EditMode reaches tmp only: a fix edits the scratch copies of the term
	// documents and the Registry, which the engine commits once they parse.
	scenCheckTrip.fixApplier.UseEditMode()

	buildStepsTrip := newScenarioTriple(aiClient, cfg, models, scenarioTripleConfigKeys{
		checklistSystem:    "templates.prompts.build_steps_checklist_system",
		checklist:          "templates.prompts.build_steps_checklist",
		fixGeneratorSystem: "templates.prompts.build_steps_fix_generator_system",
		fixGenerator:       "templates.prompts.build_steps_fix_generator",
		fixApplierSystem:   "templates.prompts.build_steps_fix_applier_system",
		fixApplier:         "templates.prompts.build_steps_fix_applier",
	})
	buildStepsTrip.fixApplier.UseEditMode()

	return triples{
		apply:      applyTrip,
		buildSteps: buildStepsTrip,
		buildCode:  buildCodeTrip,
		scenCheck:  scenCheckTrip,
	}
}
