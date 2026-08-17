package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/engine"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/generators/scenariogen"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/fs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/input"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/stepcoverage"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/console"
)

// ErrBuildTestsNotConverged is returned when the build-tests walk
// finishes with one or more scenarios still missing a corresponding
// test. Sets a non-zero CLI exit code. It wraps
// runner.ErrExpectedNonconvergence so the terminal envelope classifies it
// not_fixed (finalization OK) rather than a finalization failure — the CLI
// exit behavior is unchanged (finding 7).
var ErrBuildTestsNotConverged = fmt.Errorf(
	"one or more scenarios have no corresponding executable test: %w",
	runner.ErrExpectedNonconvergence,
)

// ErrGeneratedTestsStale signals generated test files that are not what
// the registry renders.
//
// A startup refusal rather than a finding, because everything after it
// would be measured against the wrong tree: walking a suite whose tests
// no longer transcribe the registry answers a question about a
// repository that does not exist. `--fix` writes them, so the fix is one
// flag away and the message says so.
var ErrGeneratedTestsStale = errors.New("generated test files are not what the registry renders")

// ErrFixTouchedGeneratedTests signals a fix walk that left a generated
// file changed. The applier's write roots include the whole test tree,
// so a model asked to author a step definition CAN edit the test that
// asserts the scenario — and a scenario "fixed" that way is one nothing
// checks any more.
var ErrFixTouchedGeneratedTests = errors.New("the fix walk modified a generated test file")

// BuildTestsDeps bundles what `build tests` needs at the command
// boundary.
type BuildTestsDeps struct {
	RegistryLoader               *registry.RegistryLoader
	ChecklistLoader              *checklist.ChecklistLoader
	DocResolver                  *docs.Resolver
	BuildTestsEvaluator          *validate.ChecklistEvaluator
	BuildTestsFixPromptGenerator *validate.FixPromptGenerator
	BuildTestsFixApplier         *validate.FixApplier
	UserInputCollector           *input.UserInputCollector
	TableRenderer                *runner.TableRenderer
	RunDir                       *fs.RunDirectory
}

// RunBuildTests drives `build tests`. Walks every scenario in the
// requirements registry against the build-tests checklist. Each cell's
// fix asks Claude to author the missing step definition under the
// `path:` of the suite that owns the scenario — the entry in
// `architecture.testing.suites[]` whose `service:` matches the
// scenario's. Exits non-zero if any scenario is still uncovered after
// the walk.
func RunBuildTests(
	ctx context.Context,
	deps BuildTestsDeps,
	requirementsFile, architectureFile string,
	fix bool,
) error {
	tmpDir := deps.RunDir.GetTmpOutPath()

	prep := &buildTestsPrep{
		architectureFile: architectureFile,
		registryFile:     requirementsFile,
		fix:              fix,
	}

	err := runner.Run(ctx, runner.Spec[*registry.RegistryScenario]{
		Name:          "build tests",
		ChecklistName: "build-tests",
		StoryNumber:   "",
		Fix:           fix,

		LoadItems:   loadRegistryScenarios(deps, requirementsFile),
		Prepare:     prep.run,
		PostFix:     buildTestsPostFix,
		Finalize:    finalizeBuildTests,
		GetSubject:  registry.Subject,
		OnItemStart: buildTestsOnItemStart,

		Evaluator:    deps.BuildTestsEvaluator,
		FixGenerator: deps.BuildTestsFixPromptGenerator,
		FixApplier:   deps.BuildTestsFixApplier,

		ChecklistLoader: deps.ChecklistLoader,
		DocResolver:     deps.DocResolver,
		Renderer:        deps.TableRenderer,
		UI:              runner.NewFixLoopUI(deps.UserInputCollector),
		TmpDir:          tmpDir,
	})
	// After the walk, not before it: the applier may write anywhere under
	// the test tree, so the only moment this can be checked is once it
	// has stopped writing. And regardless of how the walk ended — a walk
	// that failed is exactly when a fix turn is most likely to have
	// written something desperate, so returning early on `err` would
	// skip the check on the runs that need it most.
	clobberErr := prep.checkGeneratedTestsSurvived()

	if err != nil {
		return errors.Join(fmt.Errorf("build tests command failed: %w", err), clobberErr)
	}

	return clobberErr
}

// buildTestsPrep holds what the walk's preparation stage resolves, so
// the post-walk re-check can reuse the plan rather than rebuild it.
type buildTestsPrep struct {
	architectureFile string
	registryFile     string
	fix              bool

	repoRoot string
	plan     *scenariogen.Plan
	// walking records that preparation finished, so the post-walk
	// re-check runs only when a fix turn could actually have written
	// something. Without it a refusal raised AFTER the plan was built —
	// an unwritable file, a suite that could not answer — reports the
	// leftover drift as "the fix walk modified a generated test file",
	// which accuses a walk that never started.
	walking bool
}

// run is the Prepare hook: it renders the generated tests, then narrows
// the walk to the scenarios a suite reports as unbound.
//
// Two stages in one hook because they answer two halves of the same
// question — "are the tests this registry describes present?" and "does
// every step they run bind to something?" — and both are answerable
// without a model.
func (p *buildTestsPrep) run(
	ctx context.Context,
	scenarios []*registry.RegistryScenario,
) ([]*registry.RegistryScenario, error) {
	arch, err := architecture.Load(p.architectureFile)
	if err != nil {
		return nil, err
	}

	p.repoRoot, err = os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve the working directory: %w", err)
	}

	p.plan, err = scenariogen.BuildPlan(scenarios, arch, p.repoRoot, p.registryFile)
	if err != nil {
		return nil, err
	}

	err = p.renderTests()
	if err != nil {
		return nil, err
	}

	selected, err := p.scenariosWithGaps(ctx, arch, scenarios)
	if err != nil {
		return nil, err
	}

	p.walking = true

	return selected, nil
}

// renderTests writes the generated files under --fix, and refuses a run
// whose files are stale without it.
func (p *buildTestsPrep) renderTests() error {
	if p.fix {
		written, err := scenariogen.Write(p.plan, p.repoRoot)
		if err != nil {
			return err
		}

		if len(written) > 0 {
			console.Println(fmt.Sprintf("Generated %d test file(s) from the registry:", len(written)))

			for _, path := range written {
				console.Println("  " + path)
			}
		}

		return nil
	}

	drifted, err := scenariogen.Verify(p.plan, p.repoRoot)
	if err != nil {
		return err
	}

	if len(drifted) > 0 {
		return fmt.Errorf("%w:\n%s\n  regenerate with: true-bdd build tests --fix",
			ErrGeneratedTestsStale, scenariogen.DriftReport(drifted))
	}

	return nil
}

// scenariosWithGaps asks every suite which of its steps bind and returns
// only the scenarios that need a fix turn.
//
// A suite with no `coverage:` command cannot be asked, so its scenarios
// all go to the walk — the old behaviour, kept deliberately so declaring
// the command is an upgrade rather than a requirement.
func (p *buildTestsPrep) scenariosWithGaps(
	ctx context.Context,
	arch *architecture.Architecture,
	scenarios []*registry.RegistryScenario,
) ([]*registry.RegistryScenario, error) {
	examined, gaps, err := p.askSuites(ctx, arch)
	if err != nil {
		return nil, err
	}

	selected := make([]*registry.RegistryScenario, 0, len(scenarios))

	for _, scenario := range scenarios {
		// Narrowed on what a suite says it EXAMINED, never on the mere
		// existence of a coverage command. A suite reads its own
		// configured registry, so `--requirements <other>` asks about a
		// document the suite has never heard of — and a scenario it never
		// looked at must be walked, not assumed bound.
		if examined[scenario.ID] && len(gaps[scenario.ID]) == 0 {
			continue
		}

		selected = append(selected, scenario)
	}

	slog.Info("Selected scenarios to walk",
		"total", len(scenarios),
		"walking", len(selected),
		"answered_deterministically", len(scenarios)-len(selected),
	)

	if len(selected) < len(scenarios) {
		console.Println(fmt.Sprintf(
			"%d of %d scenario(s) already bind every step; walking %d.",
			len(scenarios)-len(selected), len(scenarios), len(selected)))
	}

	return selected, nil
}

// askSuites runs every declared coverage command and merges the answers:
// which scenarios were examined, and which of them have a gap.
func (p *buildTestsPrep) askSuites(
	ctx context.Context,
	arch *architecture.Architecture,
) (map[string]bool, map[string][]string, error) {
	// Everything checkable about every suite's command, before the first
	// subprocess: finding suite three unrunnable after suites one and two
	// have each compiled a test binary costs minutes for a verdict the
	// spec could have given immediately.
	err := stepcoverage.Validate(arch.Suites)
	if err != nil {
		return nil, nil, err
	}

	examined := map[string]bool{}
	gaps := map[string][]string{}

	for _, suite := range arch.Suites {
		if strings.TrimSpace(suite.Commands.Coverage) == "" {
			continue
		}

		answer, askErr := stepcoverage.Ask(ctx, suite, p.repoRoot)
		if askErr != nil {
			return nil, nil, askErr
		}

		for id := range answer.Examined {
			examined[id] = true
		}

		for id, steps := range answer.Gaps {
			gaps[id] = steps
		}
	}

	return examined, gaps, nil
}

// checkGeneratedTestsSurvived re-verifies the generated files after a
// fix walk. Nothing to do when the walk never started — the run refused
// during Prepare — or when no fix could have written anything.
func (p *buildTestsPrep) checkGeneratedTestsSurvived() error {
	if !p.fix || !p.walking || p.plan == nil {
		return nil
	}

	drifted, err := scenariogen.Verify(p.plan, p.repoRoot)
	if err != nil {
		return err
	}

	if len(drifted) == 0 {
		return nil
	}

	// Both, like every other refusal in this repository. The console line
	// is for whoever ran the command; stdout is also the only channel a
	// fixture can assert on — `stdout_regex` matches stdout alone, and in
	// replay the judge never runs — so a message left on stderr is one no
	// scenario can hold the engine to.
	slog.Error("A fix turn modified a generated test file",
		"files", len(drifted),
	)
	console.Println("Cannot finish: " + ErrFixTouchedGeneratedTests.Error() + ":")
	console.Println(scenariogen.DriftReport(drifted))

	return fmt.Errorf("%w:\n%s", ErrFixTouchedGeneratedTests, scenariogen.DriftReport(drifted))
}

// loadRegistryScenarios is the LoadItems factory for `build tests`.
// Reads docs/scenarios.yaml and returns one item per scenario,
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
