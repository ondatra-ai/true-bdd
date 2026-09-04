package commands

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/engine"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/fs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/input"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner"
)

// ErrBuildCodeNotConverged is returned when the build-code walk finishes with
// tests still failing after the engine's max apply attempts (non-zero exit).
// It wraps runner.ErrExpectedNonconvergence so the envelope classifies it not_fixed, not a failure (finding 7).
var ErrBuildCodeNotConverged = fmt.Errorf(
	"one or more tests still failing after max fix attempts: %w",
	runner.ErrExpectedNonconvergence,
)

// BuildCodeDeps bundles what `build code` needs at the command boundary.
// Mirrors BuildTestsDeps, plus the architecture loader and the test-runner
// dispatcher that executes frameworks and parses their JSON output.
type BuildCodeDeps struct {
	TestRunnerDispatcher        *testrunner.Dispatcher
	ChecklistLoader             *checklist.ChecklistLoader
	DocResolver                 *docs.Resolver
	BuildCodeEvaluator          *validate.ChecklistEvaluator
	BuildCodeFixPromptGenerator *validate.FixPromptGenerator
	BuildCodeFixApplier         *validate.FixApplier
	UserInputCollector          *input.UserInputCollector
	TableRenderer               *runner.TableRenderer
	RunDir                      *fs.RunDirectory
}

// RunBuildCode drives `build code`. Loads architecture.yaml, discovers
// failing tests across every declared suite, and walks each through the
// build-code checklist, editing production source under services/* until it converges.
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
		DocResolver:     deps.DocResolver,
		Renderer:        deps.TableRenderer,
		UI:              runner.NewFixLoopUI(deps.UserInputCollector),
		TmpDir:          tmpDir,
	})
	if err != nil {
		return fmt.Errorf("build code command failed: %w", err)
	}

	return nil
}

// loadFailingTests is the LoadItems factory for `build code`. Dispatches
// every declared suite to its framework runner, deduplicates suites that
// run the same command over the same tree, and returns the union of failures sorted by id.
func loadFailingTests(
	deps BuildCodeDeps,
	architectureFile string,
) func(ctx context.Context) ([]*testrunner.FailingTest, error) {
	return func(ctx context.Context) ([]*testrunner.FailingTest, error) {
		arch, err := architecture.Load(architectureFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load architecture: %w", err)
		}

		err = validateSuites(deps.TestRunnerDispatcher, arch.Testing)
		if err != nil {
			return nil, err
		}

		// Write roots come from the architecture spec, not a separate config,
		// so "edits production source, never tests" is enforced by the same
		// document that defines where production source lives.
		deps.BuildCodeFixApplier.UseWriteRoots(servicePaths(arch.Services))

		failures, err := walkSuites(ctx, deps.TestRunnerDispatcher, arch)
		if err != nil {
			return nil, err
		}

		sort.Slice(failures, func(i, j int) bool {
			return failures[i].ID < failures[j].ID
		})

		slog.Info("Total failing tests across architecture", "failures", len(failures))

		return failures, nil
	}
}

// validateSuites checks the testing block before the first subprocess:
// its framework routes to a runner and its replay command is valid — a
// pre-pass, so unrunnable work is caught before a progress line claims it.
func validateSuites(dispatcher *testrunner.Dispatcher, testing architecture.Testing) error {
	_, err := dispatcher.For(testing.Framework)
	if err != nil {
		return fmt.Errorf("%s: %w", testing.Label(), err)
	}

	err = testrunner.ValidateCommand(testing.Framework, replayCommand(testing))
	if err != nil {
		return fmt.Errorf("%s: %w", testing.Label(), err)
	}

	return nil
}

// replayCommand picks the mode `build code` runs under. Hardcoded to replay:
// `record` and `live` are declared and validated by the loader but no
// command reaches them yet.
func replayCommand(testing architecture.Testing) string {
	return testing.Commands.Replay
}

// servicePaths collects each declared service's source root, skipping
// services that declare none.
func servicePaths(services []architecture.Service) []string {
	paths := make([]string, 0, len(services))

	for _, svc := range services {
		if svc.Path != "" {
			paths = append(paths, svc.Path)
		}
	}

	return paths
}

// walkSuites dispatches every declared suite to its framework runner,
// skipping a suite whose (framework, path, config, command) another
// suite already ran. Returns the union of their failures.
func walkSuites(
	ctx context.Context,
	dispatcher *testrunner.Dispatcher,
	arch *architecture.Architecture,
) ([]*testrunner.FailingTest, error) {
	return runSuiteDiscovery(ctx, dispatcher, arch)
}

// runSuiteDiscovery dispatches one suite's config to its framework
// runner, printing progress and converting the architecture-level shape
// into the testrunner-level Config shape.
func runSuiteDiscovery(
	ctx context.Context,
	dispatcher *testrunner.Dispatcher,
	arch *architecture.Architecture,
) ([]*testrunner.FailingTest, error) {
	suite := arch.Testing

	slog.Info("Running tests", "framework", suite.Framework)

	runnerImpl, err := dispatcher.For(suite.Framework)
	if err != nil {
		return nil, fmt.Errorf("dispatch %s: %w", suite.Label(), err)
	}

	rcfg := testrunner.Config{
		Framework:  suite.Framework,
		ConfigFile: suite.ConfigFile,
		Command:    replayCommand(suite),
	}

	failures, err := runnerImpl.Discover(ctx, rcfg, "", suite.Framework)
	if err != nil {
		// Reported here, on both channels: this is the one failure the
		// command path cannot validate ahead of time. Left to cobra it
		// would surface as a raw stderr traceback under a usage dump, right after the "Running ..." progress line.
		slog.Error("Cannot run test suite",
			"suite", suite.Framework,
			"command", rcfg.Command,
			"error", err,
		)
		slog.Error("Cannot run suite", "framework", suite.Label(), "error", err)

		// Marked as reported so the generic startup refusal stays quiet: this
		// already carries a specific diagnosis on both channels, and came
		// after progress was printed, so "Refusing to start" would contradict it.
		return nil, runner.Reported(fmt.Errorf("discover %s: %w", suite.Label(), err))
	}

	slog.Info("Suite failures", "failures", len(failures), "framework", suite.Label())

	return failures, nil
}

// buildCodeOnItemStart prints the per-item progress banner.
func buildCodeOnItemStart(idx, total int, item *testrunner.FailingTest) {
	slog.Info("test", "index", idx+1, "total", total, "id", item.ID)
}

// buildCodePostFix re-runs the failing test and refreshes LastRunPassed /
// FailureOutput / LastRunAt on the item, so the engine's next Query
// iteration reads the refreshed state via the same pointer.
func buildCodePostFix(
	deps BuildCodeDeps,
) func(ctx context.Context, item *testrunner.FailingTest, applierContent string) (*testrunner.FailingTest, error) {
	return func(
		ctx context.Context,
		item *testrunner.FailingTest,
		_ string,
	) (*testrunner.FailingTest, error) {
		slog.Info("Fix applied; re-running this test in isolation")

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
