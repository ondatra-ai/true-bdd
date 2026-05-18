package commands

import (
	"context"
	"errors"
	"fmt"

	"bdd-cli/src/internal/app/engine"
	"bdd-cli/src/internal/app/generators/validate"
	"bdd-cli/src/internal/app/runner"
	"bdd-cli/src/internal/infrastructure/checklist"
	"bdd-cli/src/internal/infrastructure/fs"
	"bdd-cli/src/internal/infrastructure/input"
	"bdd-cli/src/internal/infrastructure/registry"
	"bdd-cli/src/internal/pkg/console"
)

// ErrBuildTestsNotConverged is returned when the build-tests walk
// finishes with one or more scenarios still missing a corresponding
// test. Sets a non-zero CLI exit code.
var ErrBuildTestsNotConverged = errors.New(
	"one or more scenarios have no corresponding executable test",
)

// BuildTestsDeps bundles what `build tests` needs at the command
// boundary.
type BuildTestsDeps struct {
	RegistryLoader               *registry.RegistryLoader
	ChecklistLoader              *checklist.ChecklistLoader
	BuildTestsEvaluator          *validate.ChecklistEvaluator
	BuildTestsFixPromptGenerator *validate.FixPromptGenerator
	BuildTestsFixApplier         *validate.FixApplier
	UserInputCollector           *input.UserInputCollector
	TableRenderer                *runner.TableRenderer
	RunDir                       *fs.RunDirectory
}

// RunBuildTests drives `build tests`. Walks every scenario in the
// requirements registry against the build-tests checklist. Each cell's
// fix asks Claude to author the missing test directly under
// `tests/integration/`, `tests/e2e/`, `services/backend/`, or
// `services/frontend/`. Exits non-zero if any scenario is still
// uncovered after the walk.
func RunBuildTests(
	ctx context.Context,
	deps BuildTestsDeps,
	requirementsFile string,
	fix bool,
) error {
	tmpDir := deps.RunDir.GetTmpOutPath()

	err := runner.Run(ctx, runner.Spec[*registry.RegistryScenario]{
		Name:          "build tests",
		ChecklistName: "build-tests",
		StoryNumber:   "",
		Fix:           fix,

		LoadItems:   loadRegistryScenarios(deps, requirementsFile),
		PostFix:     buildTestsPostFix,
		Finalize:    finalizeBuildTests,
		GetSubject:  registry.Subject,
		OnItemStart: buildTestsOnItemStart,

		Evaluator:    deps.BuildTestsEvaluator,
		FixGenerator: deps.BuildTestsFixPromptGenerator,
		FixApplier:   deps.BuildTestsFixApplier,

		ChecklistLoader: deps.ChecklistLoader,
		Renderer:        deps.TableRenderer,
		UI:              runner.NewFixLoopUI(deps.UserInputCollector),
		TmpDir:          tmpDir,
	})
	if err != nil {
		return fmt.Errorf("build tests command failed: %w", err)
	}

	return nil
}

// loadRegistryScenarios is the LoadItems factory for `build tests`.
// Reads docs/requirements.yaml and returns one item per scenario,
// sorted by id for deterministic output.
func loadRegistryScenarios(
	deps BuildTestsDeps,
	requirementsFile string,
) func(ctx context.Context) ([]*registry.RegistryScenario, error) {
	return func(_ context.Context) ([]*registry.RegistryScenario, error) {
		scenarios, err := deps.RegistryLoader.Load(requirementsFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load requirements registry: %w", err)
		}

		return scenarios, nil
	}
}

// buildTestsOnItemStart prints the "scenario N/M: <id> — <description>"
// banner before each item walk. The id is included so per-scenario
// progress is grep-friendly in long runs.
func buildTestsOnItemStart(idx, total int, item *registry.RegistryScenario) {
	console.Header(
		fmt.Sprintf("scenario %d/%d: %s — %s", idx+1, total, item.ID, item.Description),
		runner.SeparatorWidth,
	)
}

// buildTestsPostFix is the PostFix implementation for build-tests. The
// fix already wrote test files to disk via Claude's Write/Edit tools,
// so the item itself is unchanged — Run's next Query iteration will
// re-search the test trees from disk.
func buildTestsPostFix(
	_ context.Context,
	item *registry.RegistryScenario,
	_ string,
) (*registry.RegistryScenario, error) {
	console.Println("Fix applied — re-running test-coverage check...")

	return item, nil
}

// finalizeBuildTests is the Finalize closure for `build tests`. Returns
// a non-nil error iff the walk did not converge so the CLI exits
// non-zero on any uncovered scenario.
func finalizeBuildTests(result *engine.Result[*registry.RegistryScenario]) error {
	if result.Reason == engine.Converged {
		return nil
	}

	return ErrBuildTestsNotConverged
}
