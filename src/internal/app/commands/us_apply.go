package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"bdd-cli/src/internal/app/engine"
	"bdd-cli/src/internal/app/generators/validate"
	"bdd-cli/src/internal/app/runner"
	"bdd-cli/src/internal/infrastructure/checklist"
	"bdd-cli/src/internal/infrastructure/fs"
	"bdd-cli/src/internal/infrastructure/input"
	storyinfra "bdd-cli/src/internal/infrastructure/story"
	"bdd-cli/src/internal/infrastructure/template"
	"bdd-cli/src/internal/pkg/console"
)

const scratchRegistryFilename = "requirements.yaml"

// ApplyDeps bundles what `us apply` needs at the command boundary.
type ApplyDeps struct {
	StoryScenarioParser     *storyinfra.StoryScenarioParser
	ChecklistLoader         *checklist.ChecklistLoader
	ApplyEvaluator          *validate.ChecklistEvaluator
	ApplyFixPromptGenerator *validate.FixPromptGenerator
	ApplyFixApplier         *validate.FixApplier
	UserInputCollector      *input.UserInputCollector
	TableRenderer           *runner.TableRenderer
	RunDir                  *fs.RunDirectory
}

// RunApply drives `us apply`. Walks every acceptance criterion
// against the us-apply checklist. Each cell's fix mutates a scratch
// copy of the requirements registry via Claude's Edit tool; the
// canonical file is replaced atomically only on Converged.
func RunApply(
	ctx context.Context,
	deps ApplyDeps,
	storyNumber string,
	requirementsFile string,
	fix bool,
) error {
	tmpDir := deps.RunDir.GetTmpOutPath()
	scratchPath := filepath.Join(tmpDir, scratchRegistryFilename)

	err := runner.Run(ctx, runner.Spec[*template.ScenarioApplyData]{
		Name:          "us apply",
		ChecklistName: "us-apply",
		StoryNumber:   storyNumber,
		Fix:           fix,

		LoadItems:   loadScenarios(deps, storyNumber, requirementsFile, scratchPath),
		PostFix:     scenarioPostFix,
		Finalize:    commitApplyWalk(scratchPath, requirementsFile),
		GetSubject:  scenarioSubject,
		OnItemStart: scenarioOnItemStart,

		Evaluator:    deps.ApplyEvaluator,
		FixGenerator: deps.ApplyFixPromptGenerator,
		FixApplier:   deps.ApplyFixApplier,

		ChecklistLoader: deps.ChecklistLoader,
		Renderer:        deps.TableRenderer,
		UI:              runner.NewFixLoopUI(deps.UserInputCollector),
		TmpDir:          tmpDir,
	})
	if err != nil {
		return fmt.Errorf("us apply command failed: %w", err)
	}

	return nil
}

// loadScenarios is the LoadItems factory for `us apply`. Copies the
// canonical registry to scratch, parses the refined story file into
// one ScenarioApplyData per AC, and returns them.
func loadScenarios(
	deps ApplyDeps,
	storyNumber, requirementsFile, scratchPath string,
) func(ctx context.Context) ([]*template.ScenarioApplyData, error) {
	return func(_ context.Context) ([]*template.ScenarioApplyData, error) {
		err := runner.CopyFile(requirementsFile, scratchPath)
		if err != nil {
			return nil, fmt.Errorf("failed to seed scratch registry: %w", err)
		}

		scenarios, _, err := deps.StoryScenarioParser.ParseStoryScenarios(storyNumber, scratchPath)
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

// scenarioOnItemStart is the OnItemStart implementation for apply:
// prints the "AC N/M: <description>" banner that the BDD fixture
// (and any human watching) uses to track per-AC progress through
// the outer walk.
func scenarioOnItemStart(idx, total int, item *template.ScenarioApplyData) {
	console.Header(
		fmt.Sprintf("AC %d/%d: %s", idx+1, total, item.Description),
		runner.SeparatorWidth,
	)
}

// scenarioPostFix is the PostFix implementation for apply. The fix
// already mutated the scratch file via the Edit tool, so the item
// itself is unchanged — Run's next Query iteration will read the
// new state from disk.
func scenarioPostFix(
	_ context.Context,
	item *template.ScenarioApplyData,
	_ string,
) (*template.ScenarioApplyData, error) {
	console.Printf(
		"Fix applied to scratch %s — re-running validation...\n",
		item.RequirementsScratchPath,
	)

	return item, nil
}

// commitApplyWalk returns the Finalize closure for `us apply`. On
// Converged it atomically renames the scratch registry over the
// canonical file; every other stop reason leaves the canonical
// untouched (the scratch copy is preserved in tmpDir for
// inspection).
func commitApplyWalk(
	scratchPath, requirementsFile string,
) func(*engine.Result[*template.ScenarioApplyData]) error {
	return func(result *engine.Result[*template.ScenarioApplyData]) error {
		if result.Reason != engine.Converged {
			console.Printf(
				"One or more scenarios did not pass. Canonical %s left unchanged. Scratch: %s\n",
				requirementsFile, scratchPath,
			)

			return nil
		}

		console.Header("ALL CHECKS PASSED!", runner.SeparatorWidth)

		err := os.Rename(scratchPath, requirementsFile)
		if err != nil {
			return fmt.Errorf(
				"failed to commit scratch registry to %s: %w",
				requirementsFile, err,
			)
		}

		console.Printf("All ACs passed. %s updated from scratch.\n", requirementsFile)

		return nil
	}
}
