package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/engine"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/fs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/input"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// ErrUnknownScenarioID signals an id on the command line that names no
// registry entry. Refused before the walk: the alternative is a filter
// that silently selects nothing and reports every check passing.
var ErrUnknownScenarioID = errors.New("no scenario in the registry has id")

// ErrDuplicateScenarioID signals the same id given twice, which would
// otherwise pay for the same scenario twice and report it twice.
var ErrDuplicateScenarioID = errors.New("scenario id given more than once")

// ErrScenariosFailedCheck is a walk that ended with a Scenario failing,
// for a non-zero exit: a finding, not a crash.
var ErrScenariosFailedCheck = errors.New("one or more scenarios failed the scen-check checklist")

// ScenCheckDeps bundles what `scen check` needs at the command boundary.
type ScenCheckDeps struct {
	ValidationConcurrency       int
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
// scen-check checklist, one cell per (scenario, prompt), and fails when
// any scenario does; a run that fails puts back every document a fix wrote.
func RunScenCheck(
	ctx context.Context,
	deps ScenCheckDeps,
	requirementsFile string,
	ids []string,
	fix bool,
) error {
	ledger := &fixLedger{}

	err := runner.Run(ctx, runner.Spec[*template.ScenarioCheckData]{
		Name:          "scen check",
		ChecklistName: "scen-check",
		// Stays empty: it is a Story id, validated as a slug, and a
		// scenario id is not one. Ids travel in LoadItems instead.
		StoryID:               "",
		Fix:                   fix,
		ValidationConcurrency: deps.ValidationConcurrency,

		LoadItems:    loadCheckScenarios(deps, requirementsFile, ids, fix, ledger),
		PostFix:      scenCheckPostFix(deps, requirementsFile),
		PostCheckFix: scenCheckPostCheckFix(deps, requirementsFile),
		Finalize:     finalizeScenCheck,
		GetSubject:   scenCheckSubject,
		OnItemStart:  scenCheckOnItemStart,

		Checks:       checks.ForScenarios(),
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
		return errors.Join(err, ledger.restore())
	}

	return nil
}

// loadCheckScenarios is the LoadItems factory for `scen check`: load the
// registry, narrow it to the requested ids, and convert each survivor to
// the subject the prompts see; under --fix, beside the documents a fix writes.
func loadCheckScenarios(
	deps ScenCheckDeps,
	requirementsFile string,
	ids []string,
	fix bool,
	ledger *fixLedger,
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

		items := checkItems(scenarios, selected)

		if fix {
			err = seedFixDocuments(deps, requirementsFile, items, ledger)
			if err != nil {
				return nil, err
			}
		}

		return items, nil
	}
}

// seedFixDocuments gives every item the working copies a fix edits: the
// term documents the host has, and the Registry the walk loaded.
func seedFixDocuments(
	deps ScenCheckDeps,
	requirementsFile string,
	items []*template.ScenarioCheckData,
	ledger *fixLedger,
) error {
	tmpDir := deps.RunDir.GetTmpOutPath()

	termDocuments, err := seedTermDocuments(deps.DocResolver, tmpDir)
	if err != nil {
		return err
	}

	interfaceDocuments, err := seedInterfaceDocuments(deps.DocResolver, tmpDir)
	if err != nil {
		return err
	}

	registryDocument, err := seedFixDocument(docs.KeyScenariosYAML, requirementsFile, tmpDir)
	if err != nil {
		return err
	}

	err = ledger.keep(termDocuments...)
	if err != nil {
		return err
	}

	err = ledger.keep(interfaceDocuments...)
	if err != nil {
		return err
	}

	err = ledger.keep(registryDocument)
	if err != nil {
		return err
	}

	for _, item := range items {
		item.TermDocuments = termDocuments
		item.InterfaceDocuments = interfaceDocuments
		item.RegistryDocument = &registryDocument
	}

	return nil
}

// checkItems projects every loaded scenario onto a subject that knows
// the whole Registry, and returns the selected ones, in the loader's order.
func checkItems(scenarios, selected []*registry.RegistryScenario) []*template.ScenarioCheckData {
	all := make([]*template.ScenarioCheckData, 0, len(scenarios))
	byID := make(map[string]*template.ScenarioCheckData, len(scenarios))

	for _, scenario := range scenarios {
		item := newScenarioCheckData(scenario)
		all = append(all, item)
		byID[item.ID] = item
	}

	items := make([]*template.ScenarioCheckData, 0, len(selected))

	for _, item := range all {
		item.Registry = all
	}

	for _, scenario := range selected {
		items = append(items, byID[scenario.ID])
	}

	return items
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
	item := &template.ScenarioCheckData{}
	refreshScenario(item, scenario)

	return item
}

// refreshScenario sets the subject's own fields from a loaded registry
// entry, leaving what the walk attached to it (documents, Registry) alone.
func refreshScenario(item *template.ScenarioCheckData, scenario *registry.RegistryScenario) {
	stories := make([]template.StoryRef, 0, len(scenario.UserStories))
	for _, ref := range scenario.UserStories {
		stories = append(stories, template.StoryRef{
			Story:      ref.Story,
			ScenarioID: ref.ScenarioID,
		})
	}

	item.ID = scenario.ID
	item.Description = scenario.Description
	item.Service = scenario.Service
	item.Path = scenario.Path
	item.UserStories = stories
	item.Steps = scenario.Steps
	item.Phrases = scenario.Phrases()
}

// scenCheckSubject is the GetSubject implementation: the scenario id
// names the per-cell artifacts in tmpDir and heads the report row.
func scenCheckSubject(item *template.ScenarioCheckData) (string, string) {
	return item.ID, item.Description
}

// scenCheckOnItemStart prints the per-scenario banner a long walk is
// followed by.
func scenCheckOnItemStart(idx, total int, item *template.ScenarioCheckData) {
	slog.Info("Scenario", "index", idx+1, "total", total, "id", item.ID)
}

// scenCheckPostFix commits the documents the applier edited. A fix that
// rewrote the Registry reloads it, so the restarted walk asks about the
// Scenario as it now reads; the checks re-read the term documents themselves.
func scenCheckPostFix(
	deps ScenCheckDeps,
	requirementsFile string,
) func(context.Context, *template.ScenarioCheckData, string) (*template.ScenarioCheckData, error) {
	return func(
		_ context.Context,
		item *template.ScenarioCheckData,
		_ string,
	) (*template.ScenarioCheckData, error) {
		registryChanged, err := commitFixDocuments(item)
		if err != nil {
			return nil, err
		}

		if registryChanged {
			err = reloadScenarios(deps.RegistryLoader, requirementsFile, item.Registry)
			if err != nil {
				return nil, err
			}
		}

		return item, nil
	}
}

// reloadScenarios refreshes every subject of the walk in place from the
// committed Registry, so each item, and every check relating Scenarios,
// reads it as it now stands; the fix kept the ids (readableRegistry).
func reloadScenarios(
	loader *registry.RegistryLoader,
	requirementsFile string,
	subjects []*template.ScenarioCheckData,
) error {
	scenarios, err := loader.Load(requirementsFile)
	if err != nil {
		return fmt.Errorf("reload the registry: %w", err)
	}

	byID := make(map[string]*registry.RegistryScenario, len(scenarios))
	for _, scenario := range scenarios {
		byID[scenario.ID] = scenario
	}

	for _, subject := range subjects {
		scenario, found := byID[subject.ID]
		if !found {
			return fmt.Errorf("reload the registry: %w %s", ErrUnknownScenarioID, subject.ID)
		}

		refreshScenario(subject, scenario)
	}

	return nil
}

// finalizeScenCheck reports the verdict and fails every walk that did not
// converge, so a bare `scen check` gates by its exit code (Q10).
func finalizeScenCheck(result *engine.Result[*template.ScenarioCheckData]) error {
	switch result.Reason {
	case engine.Converged:
		slog.Info("ALL CHECKS PASSED!")

		return nil
	case engine.NotFixed:
		slog.Warn("Validation failed.")
	case engine.UserExit:
		slog.Info("Exiting.")
	case engine.MaxAttemptsExhausted:
		slog.Warn("Hit max apply attempts without convergence.")
	}

	return ErrScenariosFailedCheck
}

func scenCheckPostCheckFix(
	deps ScenCheckDeps,
	requirementsFile string,
) func(context.Context, *template.ScenarioCheckData, string) (*template.ScenarioCheckData, error) {
	return func(ctx context.Context, item *template.ScenarioCheckData, check string) (*template.ScenarioCheckData, error) {
		if check != checks.CheckPublicInterface && check != checks.CheckPhraseMappings {
			return scenCheckPostFix(deps, requirementsFile)(ctx, item, "")
		}

		for _, document := range item.TermDocuments {
			if document.Key != docs.KeyArchitectureYAML {
				continue
			}

			err := commitInterfaceFix(document, item.InterfaceDocuments)
			if err != nil {
				return nil, err
			}
		}

		return item, nil
	}
}
