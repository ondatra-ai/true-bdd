package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/engine"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/fs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/input"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	storyinfra "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/story"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// scratchRegistryFilename is the scratch copy's basename inside the run
// tmpdir. Deliberately fixed: one registry per run, and the name never
// leaves the tmpdir, so it stays stable regardless of where documents.scenarios_yaml points.
const scratchRegistryFilename = "scenarios.yaml"

// errKeyNotLineage refuses a walk's Registry in which a Scenario claims a
// criterion's lineage under another key (ADR 0014).
var errKeyNotLineage = errors.New("a Scenario must be keyed by the lineage id it claims, so the Registry is unchanged")

// ApplyDeps bundles what `us apply` needs at the command boundary.
type ApplyDeps struct {
	StoryScenarioParser     *storyinfra.StoryScenarioParser
	ChecklistLoader         *checklist.ChecklistLoader
	DocResolver             *docs.Resolver
	ApplyEvaluator          *validate.ChecklistEvaluator
	ApplyFixPromptGenerator *validate.FixPromptGenerator
	ApplyFixApplier         *validate.FixApplier
	UserInputCollector      *input.UserInputCollector
	TableRenderer           *runner.TableRenderer
	RunDir                  *fs.RunDirectory
}

// RunApply drives `us apply`. Walks every acceptance criterion against the
// us-apply checklist; each fix mutates a scratch copy of the registry, and
// commitApplyWalk (below) replaces the canonical file atomically once converged.
func RunApply(
	ctx context.Context,
	deps ApplyDeps,
	storyID string,
	requirementsFile string,
	fix bool,
) error {
	tmpDir := deps.RunDir.GetTmpOutPath()
	scratchPath := filepath.Join(tmpDir, scratchRegistryFilename)
	architecturePath := func() (string, error) {
		path, err := deps.DocResolver.Resolve(docs.KeyArchitectureYAML)
		if err != nil {
			return "", fmt.Errorf("resolve the architecture document: %w", err)
		}

		return path, nil
	}

	err := runner.Run(ctx, runner.Spec[*template.ScenarioApplyData]{
		Name:          "us apply",
		ChecklistName: "us-apply",
		StoryID:       storyID,
		Fix:           fix,

		LoadItems:   loadScenarios(deps, storyID, requirementsFile, scratchPath),
		PostFix:     scenarioPostFix,
		Finalize:    commitApplyWalk(scratchPath, requirementsFile, architecturePath),
		GetSubject:  scenarioSubject,
		OnItemStart: scenarioOnItemStart,

		Evaluator:    deps.ApplyEvaluator,
		FixGenerator: deps.ApplyFixPromptGenerator,
		FixApplier:   deps.ApplyFixApplier,

		ChecklistLoader: deps.ChecklistLoader,
		DocResolver:     deps.DocResolver,
		Renderer:        deps.TableRenderer,
		UI:              runner.NewFixLoopUI(deps.UserInputCollector),
		TmpDir:          tmpDir,
	})

	return err
}

// loadScenarios is the LoadItems factory for `us apply`. Copies the
// canonical registry to scratch, parses the refined story file into
// one ScenarioApplyData per AC, and returns them.
func loadScenarios(
	deps ApplyDeps,
	storyID, requirementsFile, scratchPath string,
) func(ctx context.Context) ([]*template.ScenarioApplyData, error) {
	return func(_ context.Context) ([]*template.ScenarioApplyData, error) {
		err := runner.CopyFile(requirementsFile, scratchPath)
		if err != nil {
			return nil, fmt.Errorf("failed to seed scratch registry: %w", err)
		}

		// The checklist's `docs:` name the architecture, never the Registry,
		// so this log line is the only place the target registry path is nameable.
		slog.Info("Seeded scratch registry", "from", requirementsFile, "to", scratchPath)

		scenarios, _, err := deps.StoryScenarioParser.ParseStoryScenarios(storyID, scratchPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse story scenarios: %w", err)
		}

		return scenarios, nil
	}
}

// scenarioSubject is the GetSubject implementation for apply
// scenarios. The lineage scenario ID disambiguates per-AC artifacts
// in tmpDir.
func scenarioSubject(item *template.ScenarioApplyData) (string, string) {
	return item.LineageScenarioID, item.Description
}

// scenarioOnItemStart is the OnItemStart implementation for apply: prints
// the "AC N/M: <description>" banner that the BDD fixture (and any human
// watching) uses to track per-AC progress.
func scenarioOnItemStart(idx, total int, item *template.ScenarioApplyData) {
	slog.Info("AC", "index", idx+1, "total", total, "description", item.Description)
}

// scenarioPostFix is the PostFix implementation for apply. The fix already
// mutated the scratch file via the Edit tool, so the item itself is
// unchanged; Run's next Query iteration reads the new state from disk.
func scenarioPostFix(
	_ context.Context,
	item *template.ScenarioApplyData,
	_ string,
) (*template.ScenarioApplyData, error) {
	slog.Info("Fix applied to scratch; re-running validation",
		"scratch", item.RequirementsScratchPath)

	return item, nil
}

// commitApplyWalk returns the Finalize closure for `us apply`. On Converged
// it atomically renames the scratch registry over the canonical file; any
// other stop reason leaves the canonical untouched, scratch kept in tmpDir for inspection.
func commitApplyWalk(
	scratchPath, requirementsFile string,
	architecturePath func() (string, error),
) func(*engine.Result[*template.ScenarioApplyData]) error {
	return func(result *engine.Result[*template.ScenarioApplyData]) error {
		if result.Reason != engine.Converged {
			slog.Warn("One or more scenarios did not pass; the canonical file is unchanged",
				"canonical", requirementsFile, "scratch", scratchPath)

			return nil
		}

		scenarios, err := registry.NewRegistryLoader().Load(scratchPath)
		if err != nil {
			return fmt.Errorf("read the walked Registry: %w", err)
		}

		err = heldLineageKeys(scenarios, result.Items)
		if err != nil {
			return err
		}

		err = heldDeclaredServices(scenarios, architecturePath, result.Items)
		if err != nil {
			return err
		}

		slog.Info("ALL CHECKS PASSED!")

		// Copy then drop the scratch: disk.Copy commits through a rename, so
		// the canonical registry is never the half-written one.
		err = disk.Copy(requirementsFile, scratchPath, disk.Shared)
		if err != nil {
			return fmt.Errorf(
				"failed to commit scratch registry to %s: %w",
				requirementsFile, err,
			)
		}

		_ = disk.Remove(scratchPath)

		slog.Info("All ACs passed; updated from scratch", "file", requirementsFile)

		return nil
	}
}

// heldLineageKeys refuses the walk's Registry when a Scenario claiming one
// of the walked criteria is keyed apart from that criterion's lineage id:
// the key never depends on a model, whatever its fix wrote.
func heldLineageKeys(scenarios []*registry.RegistryScenario, items []*template.ScenarioApplyData) error {
	walked := make(map[string]bool, len(items))
	for _, item := range items {
		walked[item.LineageScenarioID] = true
	}

	offenders := make([]string, 0)

	for _, scenario := range scenarios {
		for _, ref := range scenario.UserStories {
			if !walked[ref.ScenarioID] || scenario.ID == ref.ScenarioID {
				continue
			}

			slog.Error("Scenario keyed apart from its lineage", "scenario", scenario.ID, "lineage", ref.ScenarioID)
			offenders = append(offenders, scenario.ID+" claims "+ref.ScenarioID)
		}
	}

	if len(offenders) == 0 {
		return nil
	}

	return fmt.Errorf("%w: %s", errKeyNotLineage, strings.Join(offenders, "; "))
}
