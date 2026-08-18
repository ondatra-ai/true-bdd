//go:build bdd

// Package scenarios binds bddgo's generics to the bdd-cli suite.
//
// A generated test file calls `scenarios.New(t, "E2E-001")` with no type
// argument, which is only possible because the state type is already
// fixed here. That is the whole job: one hand-written file per suite,
// naming the state, the step registrations and the harness, so the
// hundred-odd generated files beside it say nothing but their scenario.
//
// It also owns everything a run shares. The cheap part — the flag, the
// suite, the step table — happens in Main, because the coverage guards
// need it and must not pay for a harness to answer a question about
// YAML. Everything expensive happens on the first scenario instead.
package scenarios

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/steps"
	"github.com/ondatra-ai/true-bdd/tests/libraries/bddgo"
	"github.com/ondatra-ai/true-bdd/tests/libraries/runner"
)

// -mode selects how scenarios reach the AI CLIs. Usage:
//
//	go test -tags bdd ./tests/bdd-cli/ -mode=replay
//
// live (default) — real claude/crush, no shim, today's behavior.
// record — real CLIs behind the aiproxy shim; cassettes are (re)written
// under each selected fixture's cassettes/ dir. Filter with -run.
// replay — cassettes served by the shim; real AI CLIs not required
// (the judge does not run at all). A scenario without cassettes FAILS:
// silence about an un-recorded scenario would let the suite go green
// while covering less than it claims.
//
//nolint:gochecknoglobals // test-binary flag; parsed by `go test`
var proxyMode = flag.String("mode", runner.ProxyModeLive,
	"AI CLI mode for scenarios: live, record, or replay")

const (
	// scenarioTimeout caps the CLI run alone — prep and teardown have
	// their own budgets. Deliberately tight: past five minutes a run is
	// not slow, it is wrong — a fix prompt that cannot land, or a cell
	// whose verdict no fix can move. A fixture whose invocation
	// legitimately does heavy external work overrides it with its
	// scenario's `timeout:` key.
	scenarioTimeout = 5 * time.Minute
	// judgeTimeout caps the post-run judge call. The judge gets its own
	// fresh context so it can still produce a verdict when the CLI run
	// hit scenarioTimeout.
	judgeTimeout = 5 * time.Minute
	// FixturesDir holds the project tree each scenario's Given step
	// names, relative to the suite package.
	FixturesDir = "fixtures"
	// buildTimeout caps each `go build` the harness runs.
	buildTimeout = 2 * time.Minute
	// shimPerm is the mode of the directory the aiproxy shim lands in.
	shimPerm = 0o755
)

//nolint:gochecknoglobals // process-wide suite state, built once in Main
var (
	suite    *bddgo.Suite[steps.State]
	repoRoot string

	bootOnce  sync.Once
	bootErr   error
	skipWhy   string
	harness   *steps.Harness
	teardowns []func()
)

// Main is the suite's TestMain body. It returns rather than exits so the
// caller can os.Exit after the deferred teardowns have run — TestMain
// has no *testing.T, so t.Cleanup is not available and the teardowns
// have to be explicit.
func Main(m *testing.M) int {
	// Parsed explicitly: m.Run parses too late for a flag Main reads.
	flag.Parse()

	mode := *proxyMode
	if mode != runner.ProxyModeLive && mode != runner.ProxyModeRecord && mode != runner.ProxyModeReplay {
		fmt.Fprintf(os.Stderr, "invalid -mode %q: want live, record, or replay\n", mode)

		return 1
	}

	var err error

	repoRoot, err = runner.FindRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "find repo root: %v\n", err)

		return 1
	}

	suite, err = bddgo.New[steps.State](steps.Options(repoRoot))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)

		return 1
	}

	steps.Register(suite)

	code := m.Run()

	for index := len(teardowns) - 1; index >= 0; index-- {
		teardowns[index]()
	}

	return code
}

// New returns the Run a generated test drives, bringing the harness up
// on the first call.
//
// Lazily, and that matters: a coverage guard asking whether the registry
// and the generated files agree would otherwise pay for a binary build
// and an aiproxy shim to answer a question about two files on disk.
func New(t *testing.T, scenarioID string) *bddgo.Run[steps.State] {
	t.Helper()

	ensureHarness(t)

	return suite.Scenario(t, scenarioID)
}

// CheckCoverage refuses a repository whose registry and generated tests
// disagree. Hand-written on purpose: a generated guard is one the
// generator could silence by regenerating it.
func CheckCoverage(t *testing.T) {
	t.Helper()

	suite.CheckCoverage(t, ".", repoRoot)
}

// CheckFixtureTrees refuses a fixture tree no scenario names.
func CheckFixtureTrees(t *testing.T) {
	t.Helper()

	suite.CheckFixtureTrees(t, FixturesDir, steps.FixtureName)
}

// CheckStepCoverage reports every registry step that binds to no step
// definition, and writes the report `build tests` reads.
//
// The engine asks this question by running this test rather than by
// parsing the source, because the patterns live here: some are built at
// registration rather than written as literals, so a reader of the
// source can only approximate what the resolver actually does.
func CheckStepCoverage(t *testing.T) {
	t.Helper()

	suite.ReportStepCoverage(t)
}

// ensureHarness builds everything a scenario shares, once.
func ensureHarness(t *testing.T) {
	t.Helper()

	bootOnce.Do(func() { skipWhy, bootErr = bootHarness(t) })

	if bootErr != nil {
		t.Fatalf("bring the suite up: %v", bootErr)
	}

	// A skip per scenario rather than one for the whole suite: `go test`
	// then reports how many scenarios did not run and why, instead of a
	// single skipped parent that hides the count.
	if skipWhy != "" {
		t.Skip(skipWhy)
	}
}

// bootHarness assembles the binary, the session root, the shim, the
// judge and this process's log sink. A non-empty first result is a
// reason to skip; a non-nil second is a failure.
func bootHarness(t *testing.T) (string, error) {
	mode := *proxyMode

	// Replay needs no model anywhere: the engine's turns come from
	// cassettes and the verdict comes from the recording. That is what
	// makes it runnable in CI, offline, and on a machine that has never
	// installed an agent CLI — so it must NOT skip for a missing one.
	if mode != runner.ProxyModeReplay {
		why, err := missingCLI()
		if err != nil {
			return "", err
		}

		if why != "" {
			return why, nil
		}
	}

	var shimDir string

	if mode != runner.ProxyModeLive {
		var err error

		shimDir, err = installShimDir()
		if err != nil {
			return "", err
		}
	}

	judge, err := runner.NewClaudeJudge()
	if err != nil {
		return "", fmt.Errorf("init judge: %w", err)
	}

	sessionRoot, err := runner.NewSessionRoot()
	if err != nil {
		return "", fmt.Errorf("create session root: %w", err)
	}

	t.Logf("BDD session root: %s", sessionRoot)

	// Point this process's slog at the session before the first judge
	// call. Left unconfigured, slog falls back to the log package's text
	// format on stderr and the judge's cost exists structurally nowhere.
	usage, closeLog, err := runner.InstallHarnessLogging(sessionRoot)
	if err != nil {
		return "", fmt.Errorf("install harness logging: %w", err)
	}

	teardowns = append(teardowns, closeLog)

	binPath, err := buildTrueBDD()
	if err != nil {
		return "", err
	}

	harness = &steps.Harness{
		BinPath:      binPath,
		SessionRoot:  sessionRoot,
		ShimDir:      shimDir,
		Mode:         mode,
		Judge:        judge,
		FixturesDir:  FixturesDir,
		Timeout:      scenarioTimeout,
		JudgeTimeout: judgeTimeout,
		Usage:        usage,
	}

	suite.Init(steps.NewState(harness))

	return "", writeSessionMeta(sessionRoot, mode)
}

// writeSessionMeta records what this invocation set out to run, before
// the first scenario starts, so a live report's denominator is the run's
// intent rather than its progress.
func writeSessionMeta(sessionRoot, mode string) error {
	calls, err := bddgo.ScanScenarioCalls(".")
	if err != nil {
		return fmt.Errorf("scan generated tests: %w", err)
	}

	names := make([]string, 0, len(calls))
	byFunc := make(map[string]string, len(calls))

	for _, call := range calls {
		names = append(names, call.Func)
		byFunc[call.Func] = call.ID
	}

	planned, err := runner.PlannedTests(names, flagValue("test.run"), flagValue("test.skip"))
	if err != nil {
		return fmt.Errorf("resolve planned scenarios: %w", err)
	}

	// Recorded as fixture names, not function names: every reader
	// downstream — the report server, the coverage tool, the on-disk run
	// directories — keys on the tree a scenario drives, and renaming that
	// key would be a change to all of them for no gain here.
	fixtures := make([]string, 0, len(planned))

	for _, name := range planned {
		scenario, found, lookupErr := suite.Lookup(byFunc[name])
		if lookupErr != nil {
			return fmt.Errorf("look up %s: %w", byFunc[name], lookupErr)
		}

		if !found {
			continue
		}

		fixtures = append(fixtures, steps.FixtureName(scenario))
	}

	err = runner.WriteSessionMeta(sessionRoot, mode, fixtures)
	if err != nil {
		return fmt.Errorf("write session meta: %w", err)
	}

	return nil
}

// missingCLI reports why the suite cannot run against real models: the
// judge's own CLI, or one the engine config binds a model tier to.
// Without this a `coder: "crush:…"` tier on a machine without crush
// fails deep inside a fix loop, minutes in, looking like a product bug.
func missingCLI() (string, error) {
	// The judge runs on claude regardless of which CLIs the engine
	// config routes a scenario's own turns to.
	_, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Sprintf("`claude` CLI not on $PATH: %v", err), nil
	}

	clis, err := runner.RequiredCLIs(filepath.Join(repoRoot, "true-bdd", "true-bdd.yaml"))
	if err != nil {
		return "", fmt.Errorf("read engine config: %w", err)
	}

	for _, cli := range clis {
		_, lookErr := exec.LookPath(cli)
		if lookErr != nil {
			return fmt.Sprintf(
				"engine config binds a model tier to `%s`, which is not on $PATH: %v",
				cli, lookErr), nil
		}
	}

	return "", nil
}

// installShimDir builds the aiproxy shim and installs it as claude /
// crush / codex in a temp dir. Prepended to the CLI subprocess's PATH
// (never the test process's), it intercepts every AI-CLI spawn — the
// engine's own binary resolution is untouched.
func installShimDir() (string, error) {
	dir, err := os.MkdirTemp("", "true-bdd-shim-")
	if err != nil {
		return "", fmt.Errorf("create shim dir: %w", err)
	}

	teardowns = append(teardowns, func() { _ = os.RemoveAll(dir) })

	shimDir := filepath.Join(dir, "shim")

	err = os.MkdirAll(shimDir, shimPerm)
	if err != nil {
		return "", fmt.Errorf("create shim dir: %w", err)
	}

	proxyPath := filepath.Join(shimDir, "aiproxy")

	err = goBuild(proxyPath, "./tests/libraries/aiproxy")
	if err != nil {
		return "", err
	}

	for _, name := range []string{"claude", "crush", "codex"} {
		err = os.Symlink(proxyPath, filepath.Join(shimDir, name))
		if err != nil {
			return "", fmt.Errorf("install shim %s: %w", name, err)
		}
	}

	return shimDir, nil
}

func buildTrueBDD() (string, error) {
	dir, err := os.MkdirTemp("", "true-bdd-bin-")
	if err != nil {
		return "", fmt.Errorf("create bin dir: %w", err)
	}

	teardowns = append(teardowns, func() { _ = os.RemoveAll(dir) })

	binPath := filepath.Join(dir, "true-bdd")

	err = goBuild(binPath, "./services/bdd-cli")
	if err != nil {
		return "", err
	}

	return binPath, nil
}

// goBuild builds one package of the repository module into binPath.
func goBuild(binPath, pkg string) error {
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build", "-C", repoRoot, "-o", binPath, pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("go build %s: %w", pkg, err)
	}

	return nil
}

// flagValue reads one of `go test`'s own flags. Absent reads as empty:
// a filter that is not set filters nothing.
func flagValue(name string) string {
	found := flag.Lookup(name)
	if found == nil {
		return ""
	}

	return found.Value.String()
}
