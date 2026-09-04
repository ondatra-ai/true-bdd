package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/console"
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
)

// ErrBuildTestsNotConverged is returned when the build-tests walk finishes
// with scenarios still missing a test, for a non-zero exit code. It wraps
// runner.ErrExpectedNonconvergence so the envelope classifies it not_fixed, not a failure (finding 7).
var ErrBuildTestsNotConverged = fmt.Errorf(
	"one or more scenarios have no corresponding executable test: %w",
	runner.ErrExpectedNonconvergence,
)

// ErrGeneratedTestsStale signals generated test files that are not what the
// registry renders. Refused at startup, not as a per-scenario finding: every
// scenario walked after would be measured against the wrong tree; --fix regenerates them.
var ErrGeneratedTestsStale = errors.New("generated test files are not what the registry renders")

// ErrFixTouchedGeneratedTests signals a fix walk that left a generated file
// changed. The applier's write roots cover the whole test tree, so a step
// fix CAN edit the test asserting the scenario, and nothing else re-checks it.
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

// RunBuildTests drives `build tests`: walks every scenario in the
// requirements registry against the build-tests checklist, fixing each gap
// via its owning suite. Exits non-zero if any scenario is still uncovered.
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
	// Runs unconditionally, even when err != nil: a failed walk is exactly
	// when a desperate fix turn is most likely to have touched a generated
	// file, so returning early on `err` would skip the check when it matters most.
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
	// walking marks that Prepare finished, so the post-walk re-check runs
	// only once a fix turn could actually have written something — without
	// it, a refusal raised after the plan builds would misreport as clobbering.
	walking bool
}

// run is the Prepare hook: it renders the generated tests, then narrows the
// walk to the scenarios a suite reports as unbound — both answerable without a model.
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
			slog.Info("Generated test files from the registry",
				"count", len(written), "files", strings.Join(written, " "))
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
// only the scenarios needing a fix turn. A suite with no `coverage:` command
// sends all its scenarios to the walk rather than skipping them.
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
		// Gated on EXAMINED, not merely having a coverage command: a suite
		// reads its own configured registry, so a scenario from a different
		// `--requirements <other>` was never examined and must still be walked.
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
	// Everything checkable about every suite's command is checked before the
	// first subprocess, so an unrunnable suite is reported immediately rather
	// than after earlier suites have already paid to compile a test binary.
	err := stepcoverage.Validate(arch.Testing)
	if err != nil {
		return nil, nil, err
	}

	examined := map[string]bool{}
	gaps := map[string][]string{}

	if strings.TrimSpace(arch.Testing.Commands.Coverage) != "" {
		answer, askErr := stepcoverage.Ask(ctx, arch.Testing, serviceNames(arch.Services), p.repoRoot)
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

// serviceNames is every service the architecture declares, which is what
// a coverage report's `suite` field must name.
func serviceNames(services []architecture.Service) []string {
	names := make([]string, 0, len(services))
	for _, svc := range services {
		names = append(names, svc.Name)
	}

	return names
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

	// Both: the console line is for whoever ran the command, and stdout is
	// the only channel a fixture can assert on (`stdout_regex`, replay mode)
	// — a message left on stderr alone is one no scenario can hold the engine to.
	slog.Error("A fix turn modified a generated test file",
		"files", len(drifted),
	)
	slog.Error("Cannot finish",
		"error", ErrFixTouchedGeneratedTests, "drift", scenariogen.DriftReport(drifted))

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
	slog.Info("scenario",
		"index", idx+1, "total", total, "id", item.ID, "description", item.Description)
}

// buildTestsPostFix is the PostFix implementation for build-tests. The fix
// already wrote test files to disk via Claude's tools, so the item itself is
// unchanged; Run's next Query iteration re-reads the test trees from disk.
func buildTestsPostFix(
	_ context.Context,
	item *registry.RegistryScenario,
	_ string,
) (*registry.RegistryScenario, error) {
	slog.Info("Fix applied; re-running test-coverage check")

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
