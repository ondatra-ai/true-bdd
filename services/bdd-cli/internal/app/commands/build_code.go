package commands

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
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
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/console"
)

// ErrBuildCodeNotConverged is returned when the build-code walk
// finishes with one or more tests still failing after the engine's
// max apply attempts. Sets a non-zero CLI exit code. It wraps
// runner.ErrExpectedNonconvergence so the terminal envelope classifies it
// not_fixed (finalization OK) rather than a finalization failure — the CLI
// exit behavior is unchanged (finding 7).
var ErrBuildCodeNotConverged = fmt.Errorf(
	"one or more tests still failing after max fix attempts: %w",
	runner.ErrExpectedNonconvergence,
)

// BuildCodeDeps bundles what `build code` needs at the command
// boundary. Mirrors BuildTestsDeps; the new entries are the architecture
// loader (drives scope) and the test-runner dispatcher (executes
// frameworks and parses their JSON output).
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
// failing tests across every declared test suite, and walks each
// through the build-code checklist. With fix=true, each failing cell's
// Claude turn edits production source under services/* until the engine
// converges. Exits non-zero if any test is still failing after the walk.
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

// loadFailingTests is the LoadItems factory for `build code`. Loads
// architecture.yaml, dispatches every declared test suite to its
// framework runner, deduplicates suites that run the same command over
// the same tree, and returns the union of failures sorted by id for
// deterministic walk order.
func loadFailingTests(
	deps BuildCodeDeps,
	architectureFile string,
) func(ctx context.Context) ([]*testrunner.FailingTest, error) {
	return func(ctx context.Context) ([]*testrunner.FailingTest, error) {
		arch, err := architecture.Load(architectureFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load architecture: %w", err)
		}

		err = validateSuites(deps.TestRunnerDispatcher, arch.Suites)
		if err != nil {
			return nil, err
		}

		// The applier may write exactly the production roots the
		// architecture declares. Taken from the spec rather than config
		// so "edits production source, never tests" is enforced by the
		// same document that defines where production source lives.
		deps.BuildCodeFixApplier.UseWriteRoots(servicePaths(arch.Services))

		failures, err := walkSuites(ctx, deps.TestRunnerDispatcher, arch)
		if err != nil {
			return nil, err
		}

		sort.Slice(failures, func(i, j int) bool {
			return failures[i].ID < failures[j].ID
		})

		console.Println(fmt.Sprintf("Total failing tests across architecture: %d", len(failures)))

		return failures, nil
	}
}

// validateSuites checks every declared suite before the first
// subprocess is spawned: that its framework routes to a runner, and
// that its replay command is one that runner can act on.
//
// A pre-pass rather than a check at each suite's turn: discovery runs a
// whole test suite at a time, so finding the second suite unrunnable
// after the first has already run costs minutes for a verdict the spec
// could have given immediately — and leaves behind a run that did half
// its work. The framework check belongs here for a second reason: at
// its turn, the walk has already printed "Running calc tests via
// rspec...", claiming work that cannot start.
func validateSuites(dispatcher *testrunner.Dispatcher, suites []architecture.Suite) error {
	for _, suite := range suites {
		err := validateSuite(dispatcher, suite)
		if err != nil {
			return fmt.Errorf("%s: %w", suite.Label(), err)
		}
	}

	return nil
}

// validateSuite reports the first thing that makes one declared suite
// unrunnable.
func validateSuite(dispatcher *testrunner.Dispatcher, suite architecture.Suite) error {
	_, err := dispatcher.For(suite.Framework)
	if err != nil {
		return err
	}

	return testrunner.ValidateCommand(suite.Framework, replayCommand(suite))
}

// replayCommand picks the mode `build code` runs under. Hardcoded:
// `record` and `live` are declared and validated by the loader but no
// command reaches them yet, so naming replay here — once — is the whole
// of the mode selection.
func replayCommand(suite architecture.Suite) string {
	return suite.Commands.Replay
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
	seen := make(map[string]bool)
	out := make([]*testrunner.FailingTest, 0)

	for _, suite := range arch.Suites {
		// The command joins the key because it is what actually runs:
		// two suites agreeing on framework, path and config but
		// declaring different commands are different work, and
		// collapsing them would silently drop one of them.
		dedupKey := strings.Join([]string{
			suite.Framework,
			suite.Path,
			suite.ConfigFile,
			replayCommand(suite),
		}, "\x00")

		if seen[dedupKey] {
			console.Println(fmt.Sprintf("Skipping %s (already covered by another suite)", suite.Label()))

			continue
		}

		seen[dedupKey] = true

		failures, runErr := runSuiteDiscovery(ctx, dispatcher, suite)
		if runErr != nil {
			return nil, runErr
		}

		out = append(out, failures...)
	}

	return out, nil
}

// runSuiteDiscovery dispatches one suite's config to its framework
// runner, printing progress and converting the architecture-level shape
// into the testrunner-level Config shape.
func runSuiteDiscovery(
	ctx context.Context,
	dispatcher *testrunner.Dispatcher,
	suite architecture.Suite,
) ([]*testrunner.FailingTest, error) {
	console.Println(fmt.Sprintf("Running %s tests via %s...", suite.Label(), suite.Framework))

	runnerImpl, err := dispatcher.For(suite.Framework)
	if err != nil {
		return nil, fmt.Errorf("dispatch %s: %w", suite.Label(), err)
	}

	rcfg := testrunner.Config{
		Path:       suite.Path,
		Framework:  suite.Framework,
		ConfigFile: suite.ConfigFile,
		Pattern:    suite.Pattern,
		Command:    replayCommand(suite),
	}

	failures, err := runnerImpl.Discover(ctx, rcfg, suite.Service, suite.Name)
	if err != nil {
		// Reported here, on both channels, because this is the one
		// failure in the command path no validation can reach: the spec
		// can be complete, splittable and parseable and the binary
		// still absent. Left to cobra it would surface as a stderr
		// traceback under a usage dump, right after a progress line
		// saying the suite was running.
		slog.Error("Cannot run test suite",
			"suite", suite.Name,
			"service", suite.Service,
			"command", rcfg.Command,
			"error", err,
		)
		console.Println(fmt.Sprintf("Cannot run %s: %s", suite.Label(), err.Error()))

		// Marked as reported so the generic startup refusal stays quiet:
		// this failure is already on both channels with a more specific
		// diagnosis, and it happened after the run started, so a second
		// line headlined "Refusing to start" would contradict the
		// progress line printed just above.
		return nil, runner.Reported(fmt.Errorf("discover %s: %w", suite.Label(), err))
	}

	console.Println(fmt.Sprintf("  %d failure(s) in %s", len(failures), suite.Label()))

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
