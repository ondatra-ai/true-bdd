package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/engine"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/fs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/input"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/template"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/console"
)

// ErrUnknownScenarioID signals an id on the command line that names no
// registry entry. Refused before the walk: the alternative is a filter
// that silently selects nothing and reports every check passing.
var ErrUnknownScenarioID = errors.New("no scenario in the registry has id")

// ErrDuplicateScenarioID signals the same id given twice, which would
// otherwise pay for the same scenario twice and report it twice.
var ErrDuplicateScenarioID = errors.New("scenario id given more than once")

// ScenCheckDeps bundles what `scen check` needs at the command boundary.
type ScenCheckDeps struct {
	RegistryLoader              *registry.RegistryLoader
	ChecklistLoader             *checklist.ChecklistLoader
	DocResolver                 *docs.Resolver
	ScenCheckEvaluator          *validate.ChecklistEvaluator
	ScenCheckFixPromptGenerator *validate.FixPromptGenerator
	ScenCheckFixApplier         *validate.FixApplier
	UserInputCollector          *input.UserInputCollector
	TableRenderer               *runner.TableRenderer
	RunDir                      *fs.RunDirectory
}

// RunScenCheck drives `scen check`: walks registry scenarios against the
// scen-check checklist, one cell per (scenario, prompt). Advisory — a
// failed walk still exits 0, per docs/adr/0001.
func RunScenCheck(
	ctx context.Context,
	deps ScenCheckDeps,
	requirementsFile string,
	ids []string,
	fix bool,
) error {
	err := runner.Run(ctx, runner.Spec[*template.ScenarioCheckData]{
		Name:          "scen check",
		ChecklistName: "scen-check",
		// Stays empty: validateStoryNumber expects the `4.1` shape and
		// would reject a scenario id. Ids travel in LoadItems instead.
		StoryNumber: "",
		Fix:         fix,

		LoadItems:   loadCheckScenarios(deps, requirementsFile, ids),
		PostFix:     scenCheckPostFix,
		Finalize:    finalizeScenCheck,
		GetSubject:  scenCheckSubject,
		OnItemStart: scenCheckOnItemStart,

		Evaluator:    deps.ScenCheckEvaluator,
		FixGenerator: deps.ScenCheckFixPromptGenerator,
		FixApplier:   deps.ScenCheckFixApplier,

		ChecklistLoader: deps.ChecklistLoader,
		DocResolver:     deps.DocResolver,
		Renderer:        deps.TableRenderer,
		UI:              runner.NewFixLoopUI(deps.UserInputCollector),
		TmpDir:          deps.RunDir.GetTmpOutPath(),
	})
	if err != nil {
		return fmt.Errorf("scen check command failed: %w", err)
	}

	return nil
}

// loadCheckScenarios is the LoadItems factory for `scen check`: load the
// registry, narrow it to the requested ids, and convert each survivor to
// the subject the prompts see.
func loadCheckScenarios(
	deps ScenCheckDeps,
	requirementsFile string,
	ids []string,
) func(ctx context.Context) ([]*template.ScenarioCheckData, error) {
	return func(_ context.Context) ([]*template.ScenarioCheckData, error) {
		// Through the loader, so an empty registry raises its refusal
		// rather than a walk over nothing — and so the ascending id sort
		// replay depends on comes from the one place that does it.
		scenarios, err := deps.RegistryLoader.Load(requirementsFile)
		if err != nil {
			return nil, err
		}

		selected, err := selectByID(scenarios, ids)
		if err != nil {
			return nil, err
		}

		items := make([]*template.ScenarioCheckData, 0, len(selected))
		for _, scenario := range selected {
			items = append(items, newScenarioCheckData(scenario))
		}

		return items, nil
	}
}

// selectByID narrows the loaded scenarios to the ids given on the command
// line, preserving the loader's ascending order whatever order they were
// typed in. No ids means every scenario.
func selectByID(
	scenarios []*registry.RegistryScenario,
	ids []string,
) ([]*registry.RegistryScenario, error) {
	if len(ids) == 0 {
		return scenarios, nil
	}

	wanted := make(map[string]bool, len(ids))

	for _, id := range ids {
		if wanted[id] {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateScenarioID, id)
		}

		wanted[id] = true
	}

	selected := make([]*registry.RegistryScenario, 0, len(ids))

	for _, scenario := range scenarios {
		if wanted[scenario.ID] {
			selected = append(selected, scenario)
			delete(wanted, scenario.ID)
		}
	}

	for _, id := range ids {
		if wanted[id] {
			return nil, fmt.Errorf("%w %s", ErrUnknownScenarioID, id)
		}
	}

	return selected, nil
}

// newScenarioCheckData projects a loaded registry entry onto the subject
// the prompts see: the scenario's own fields, and no path to the file
// they came from.
func newScenarioCheckData(scenario *registry.RegistryScenario) *template.ScenarioCheckData {
	stories := make([]template.StoryRef, 0, len(scenario.UserStories))
	for _, ref := range scenario.UserStories {
		stories = append(stories, template.StoryRef{
			Story:      ref.Story,
			ScenarioID: ref.ScenarioID,
		})
	}

	return &template.ScenarioCheckData{
		ID:          scenario.ID,
		Description: scenario.Description,
		Service:     scenario.Service,
		Path:        scenario.Path,
		UserStories: stories,
		Steps:       scenario.Steps,
	}
}

// scenCheckSubject is the GetSubject implementation: the scenario id
// names the per-cell artifacts in tmpDir and heads the report row.
func scenCheckSubject(item *template.ScenarioCheckData) (string, string) {
	return item.ID, item.Description
}

// scenCheckOnItemStart prints the per-scenario banner a long walk is
// followed by.
func scenCheckOnItemStart(idx, total int, item *template.ScenarioCheckData) {
	console.Header(
		fmt.Sprintf("Scenario %d/%d: %s", idx+1, total, item.ID),
		runner.SeparatorWidth,
	)
}

// scenCheckPostFix is unreachable while no prompt carries an `F:` —
// --fix is refused at startup. It returns the item untouched rather than
// staying nil, which runner.Run's apply closure would dereference.
func scenCheckPostFix(
	_ context.Context,
	item *template.ScenarioCheckData,
	_ string,
) (*template.ScenarioCheckData, error) {
	return item, nil
}

// finalizeScenCheck reports the verdict and returns nil for every stop
// reason: the command is advisory, so a failed walk still exits 0 (ADR
// 0001). Only build tests/code wrap ErrExpectedNonconvergence.
func finalizeScenCheck(result *engine.Result[*template.ScenarioCheckData]) error {
	console.BlankLine()

	switch result.Reason {
	case engine.Converged:
		console.Header("ALL CHECKS PASSED!", runner.SeparatorWidth)
	case engine.NotFixed:
		console.Println("Validation failed.")
	case engine.UserExit:
		console.Println("Exiting.")
	case engine.MaxAttemptsExhausted:
		console.Println("Hit max apply attempts without convergence.")
	}

	return nil
}
