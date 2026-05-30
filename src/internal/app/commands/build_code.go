package commands

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"bdd-cli/src/internal/app/engine"
	"bdd-cli/src/internal/app/generators/validate"
	"bdd-cli/src/internal/app/runner"
	"bdd-cli/src/internal/infrastructure/architecture"
	"bdd-cli/src/internal/infrastructure/checklist"
	"bdd-cli/src/internal/infrastructure/fs"
	"bdd-cli/src/internal/infrastructure/input"
	"bdd-cli/src/internal/infrastructure/testrunner"
	"bdd-cli/src/internal/pkg/console"
)

// ErrBuildCodeNotConverged is returned when the build-code walk
// finishes with one or more tests still failing after the engine's
// max apply attempts. Sets a non-zero CLI exit code.
var ErrBuildCodeNotConverged = errors.New(
	"one or more tests still failing after max fix attempts",
)

// BuildCodeDeps bundles what `build code` needs at the command
// boundary. Mirrors BuildTestsDeps; the new entries are the architecture
// loader (drives scope) and the test-runner dispatcher (executes
// frameworks and parses their JSON output).
type BuildCodeDeps struct {
	ArchitectureLoader          *architecture.Loader
	TestRunnerDispatcher        *testrunner.Dispatcher
	ChecklistLoader             *checklist.ChecklistLoader
	BuildCodeEvaluator          *validate.ChecklistEvaluator
	BuildCodeFixPromptGenerator *validate.FixPromptGenerator
	BuildCodeFixApplier         *validate.FixApplier
	UserInputCollector          *input.UserInputCollector
	TableRenderer               *runner.TableRenderer
	RunDir                      *fs.RunDirectory
}

// RunBuildCode drives `build code`. Loads architecture.yaml, discovers
// failing tests across every declared (service, layer) pair, and walks
// each through the build-code checklist. With fix=true, each failing
// cell's Claude turn edits production source under services/* until the
// engine converges. Exits non-zero if any test is still failing after
// the walk.
func RunBuildCode(
	ctx context.Context,
	deps BuildCodeDeps,
	architectureFile string,
	fix bool,
) error {
	tmpDir := deps.RunDir.GetTmpOutPath()

	err := runner.Run(ctx, runner.Spec[*testrunner.FailingTest]{
		Name:          "build code",
		ChecklistName: "build-code",
		StoryNumber:   "",
		Fix:           fix,

		LoadItems:   loadFailingTests(deps, architectureFile),
		PostFix:     buildCodePostFix(deps),
		Finalize:    finalizeBuildCode,
		GetSubject:  testrunner.Subject,
		OnItemStart: buildCodeOnItemStart,

		Evaluator:    deps.BuildCodeEvaluator,
		FixGenerator: deps.BuildCodeFixPromptGenerator,
		FixApplier:   deps.BuildCodeFixApplier,

		ChecklistLoader: deps.ChecklistLoader,
		Renderer:        deps.TableRenderer,
		UI:              runner.NewFixLoopUI(deps.UserInputCollector),
		TmpDir:          tmpDir,
	})
	if err != nil {
		return fmt.Errorf("build code command failed: %w", err)
	}

	return nil
}

// loadFailingTests is the LoadItems factory for `build code`. Loads
// architecture.yaml, iterates every (service, layer) block,
// deduplicates by (framework, path, configFile), dispatches each block
// to its framework runner, and returns the union of failures sorted by
// id for deterministic walk order.
func loadFailingTests(
	deps BuildCodeDeps,
	architectureFile string,
) func(ctx context.Context) ([]*testrunner.FailingTest, error) {
	return func(ctx context.Context) ([]*testrunner.FailingTest, error) {
		arch, err := deps.ArchitectureLoader.Load(architectureFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load architecture: %w", err)
		}

		seen := make(map[string]bool)
		failures := make([]*testrunner.FailingTest, 0)

		for _, svc := range arch.Services {
			batch, walkErr := walkServiceLayers(ctx, deps.TestRunnerDispatcher, svc, seen)
			if walkErr != nil {
				return nil, walkErr
			}

			failures = append(failures, batch...)
		}

		sort.Slice(failures, func(i, j int) bool {
			return failures[i].ID < failures[j].ID
		})

		console.Println(fmt.Sprintf("Total failing tests across architecture: %d", len(failures)))

		return failures, nil
	}
}

// walkServiceLayers iterates the three test layers declared by one
// service, dispatching each to its framework runner and skipping
// (framework, path, configFile) combinations already discovered through
// another service entry. Returns the failures collected for this
// service's layers.
func walkServiceLayers(
	ctx context.Context,
	dispatcher *testrunner.Dispatcher,
	svc architecture.Service,
	seen map[string]bool,
) ([]*testrunner.FailingTest, error) {
	layers := []struct {
		name string
		cfg  architecture.TestConfig
	}{
		{testrunner.LayerUnit, svc.Tests.Unit},
		{testrunner.LayerIntegration, svc.Tests.Integration},
		{testrunner.LayerE2E, svc.Tests.E2E},
	}

	out := make([]*testrunner.FailingTest, 0)

	for _, layer := range layers {
		if layer.cfg.Framework == "" {
			continue
		}

		dedupKey := layer.cfg.Framework + "\x00" + layer.cfg.Path + "\x00" + layer.cfg.ConfigFile
		if seen[dedupKey] {
			console.Println(fmt.Sprintf("Skipping %s/%s (already covered by another service)", svc.Name, layer.name))

			continue
		}

		seen[dedupKey] = true

		failures, runErr := runLayerDiscovery(ctx, dispatcher, svc.Name, layer.name, layer.cfg)
		if runErr != nil {
			return nil, runErr
		}

		out = append(out, failures...)
	}

	return out, nil
}

// runLayerDiscovery dispatches one layer's test config to its framework
// runner, printing progress and converting the architecture-level config
// shape into the testrunner-level Config shape.
func runLayerDiscovery(
	ctx context.Context,
	dispatcher *testrunner.Dispatcher,
	service, layer string,
	cfg architecture.TestConfig,
) ([]*testrunner.FailingTest, error) {
	console.Println(fmt.Sprintf("Running %s/%s tests via %s...", service, layer, cfg.Framework))

	runnerImpl, err := dispatcher.For(cfg.Framework)
	if err != nil {
		return nil, fmt.Errorf("dispatch %s/%s: %w", service, layer, err)
	}

	rcfg := testrunner.Config{
		Path:       cfg.Path,
		Framework:  cfg.Framework,
		ConfigFile: cfg.ConfigFile,
		Pattern:    cfg.Pattern,
	}

	failures, err := runnerImpl.Discover(ctx, rcfg, service, layer)
	if err != nil {
		return nil, fmt.Errorf("discover %s/%s: %w", service, layer, err)
	}

	console.Println(fmt.Sprintf("  %d failure(s) in %s/%s", len(failures), service, layer))

	return failures, nil
}

// buildCodeOnItemStart prints the per-item progress banner.
func buildCodeOnItemStart(idx, total int, item *testrunner.FailingTest) {
	console.Header(
		fmt.Sprintf("test %d/%d: %s", idx+1, total, item.ID),
		runner.SeparatorWidth,
	)
}

// buildCodePostFix re-runs the failing test through its framework
// runner and refreshes LastRunPassed / FailureOutput / LastRunAt on the
// item. Returning the same pointer lets the engine's next Query
// iteration read the refreshed state without a separate channel.
func buildCodePostFix(
	deps BuildCodeDeps,
) func(ctx context.Context, item *testrunner.FailingTest, applierContent string) (*testrunner.FailingTest, error) {
	return func(
		ctx context.Context,
		item *testrunner.FailingTest,
		_ string,
	) (*testrunner.FailingTest, error) {
		console.Println("Fix applied — re-running this test in isolation...")

		runnerImpl, err := deps.TestRunnerDispatcher.For(item.Framework)
		if err != nil {
			return item, fmt.Errorf("postfix dispatch %s: %w", item.Framework, err)
		}

		passed, output, runErr := runnerImpl.RunOne(ctx, item)
		if runErr != nil {
			return item, fmt.Errorf("postfix rerun %s: %w", item.ID, runErr)
		}

		item.LastRunPassed = passed
		item.FailureOutput = testrunner.TruncateTail(output, testrunner.FailureOutputCap)
		item.LastRunAt = time.Now()

		return item, nil
	}
}

// finalizeBuildCode is the Finalize closure for `build code`. Non-nil
// error iff the walk did not converge so the CLI exits non-zero on any
// still-failing test.
func finalizeBuildCode(result *engine.Result[*testrunner.FailingTest]) error {
	if result.Reason == engine.Converged {
		return nil
	}

	return ErrBuildCodeNotConverged
}
