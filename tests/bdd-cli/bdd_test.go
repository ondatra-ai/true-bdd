//go:build bdd

package bdd_test

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
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
	// whose verdict no fix can move. The engine bounds its own fix loop
	// and fails on an applier that wrote nothing, so a run that still
	// overruns this is a bug worth failing fast on rather than paying
	// thirty minutes to confirm. A fixture whose invocation legitimately
	// does heavy external work (building a Docker image, re-running a
	// browser suite) overrides it with `timeout:` in its manifest.
	scenarioTimeout = 5 * time.Minute
	// judgeTimeout caps the post-run judge call. The judge gets its own
	// fresh context so it can still produce a verdict when the CLI run
	// hit scenarioTimeout (otherwise the same expired context would
	// short-circuit the judge with "context deadline exceeded" and mask
	// the real "CLI was killed" failure).
	judgeTimeout = 5 * time.Minute
	// fixturesDir holds the project tree each scenario's Given step
	// names, relative to this package.
	fixturesDir = "fixtures"
)

// TestBDDFixtures runs every scenario in docs/scenarios.yaml that the
// architectural spec assigns to the bdd-cli suite.
//
// The name is unchanged, and so is each subtest's: a scenario is named
// after the project tree it drives, which is what every `-run` filter in
// this repo's gates, CI job and record hints already types. What moved
// is where the suite's contents come from — the registry, not a
// directory listing.
func TestBDDFixtures(t *testing.T) {
	mode := *proxyMode
	if mode != runner.ProxyModeLive && mode != runner.ProxyModeRecord && mode != runner.ProxyModeReplay {
		t.Fatalf("invalid -mode %q: want live, record, or replay", mode)
	}

	// Replay needs no model anywhere: the engine's turns come from
	// cassettes and the verdict comes from the recording. That is what
	// makes it runnable in CI, offline, and on a machine that has never
	// installed an agent CLI — so it must NOT skip for a missing one.
	if mode != runner.ProxyModeReplay {
		// The judge runs on claude regardless of which CLIs the engine
		// config routes a scenario's own turns to.
		_, err := exec.LookPath("claude")
		if err != nil {
			t.Skipf("`claude` CLI not on $PATH; skipping BDD suite: %v", err)
		}

		requireConfiguredCLIs(t)
	}

	harness := buildHarness(t, mode)

	repoRoot, err := runner.FindRepoRoot()
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}

	suite := bddgo.New[steps.State](t, steps.Options(repoRoot))
	suite.Init(steps.NewState(harness))
	steps.Register(suite)

	scenarios, err := suite.Scenarios()
	if err != nil {
		t.Fatalf("%v", err)
	}

	names := scenarioNames(scenarios)

	checkTreesArePaired(t, names)

	// Written before the first scenario starts: the report's denominator
	// is what this invocation set out to run, not what it has finished.
	planned, err := runner.PlannedFixtures("TestBDDFixtures", names,
		flagValue("test.run"), flagValue("test.skip"))
	if err != nil {
		t.Fatalf("resolve planned scenarios: %v", err)
	}

	err = runner.WriteSessionMeta(harness.SessionRoot, mode, planned)
	if err != nil {
		t.Fatalf("write session meta: %v", err)
	}

	suite.RunScenarios(scenarios)
}

// buildHarness assembles what every scenario shares: the binary, the
// session root, the shim, the judge, and the harness process's own log
// sink.
func buildHarness(t *testing.T, mode string) *steps.Harness {
	t.Helper()

	var shimDir string
	if mode != runner.ProxyModeLive {
		shimDir = installShimDir(t)
	}

	judge, err := runner.NewClaudeJudge()
	if err != nil {
		t.Fatalf("init judge: %v", err)
	}

	sessionRoot, err := runner.NewSessionRoot()
	if err != nil {
		t.Fatalf("create session root: %v", err)
	}

	t.Logf("BDD session root: %s", sessionRoot)

	// Point this process's slog at the session before the first judge
	// call. Left unconfigured, slog falls back to the log package's text
	// format on stderr and the judge's cost exists structurally nowhere.
	usage, closeLog, err := runner.InstallHarnessLogging(sessionRoot)
	if err != nil {
		t.Fatalf("install harness logging: %v", err)
	}

	t.Cleanup(closeLog)

	return &steps.Harness{
		BinPath:      buildTrueBDD(t),
		SessionRoot:  sessionRoot,
		ShimDir:      shimDir,
		Mode:         mode,
		Judge:        judge,
		FixturesDir:  fixturesDir,
		Timeout:      scenarioTimeout,
		JudgeTimeout: judgeTimeout,
		Usage:        usage,
	}
}

// scenarioNames is the subtest name of each scenario, in run order.
func scenarioNames(scenarios []bddgo.Scenario) []string {
	names := make([]string, 0, len(scenarios))
	for _, scenario := range scenarios {
		names = append(names, steps.FixtureName(scenario))
	}

	return names
}

// checkTreesArePaired refuses a run in which the registry and the
// fixtures directory disagree.
//
// Both directions, because both are silent. A scenario naming a tree
// that does not exist fails loudly on its own — but a TREE with no
// scenario does not: it simply stops being run, and a directory nobody
// executes looks exactly like a directory that passes. Renaming a
// fixture without updating the registry is the way that happens.
func checkTreesArePaired(t *testing.T, named []string) {
	t.Helper()

	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}

	claimed := make(map[string]bool, len(named))
	for _, name := range named {
		claimed[name] = true
	}

	var orphans []string

	for _, entry := range entries {
		if !entry.IsDir() || claimed[entry.Name()] {
			continue
		}

		orphans = append(orphans, entry.Name())
	}

	if len(orphans) == 0 {
		return
	}

	sort.Strings(orphans)
	t.Fatalf("%d fixture tree(s) no scenario names, so nothing runs them: %s\n"+
		"add a scenario to docs/scenarios.yaml for each, or delete the tree",
		len(orphans), strings.Join(orphans, ", "))
}

// installShimDir builds the aiproxy shim and installs it as claude /
// crush / codex in a temp dir. Prepended to the CLI subprocess's PATH
// (never the test process's), it intercepts every AI-CLI spawn — the
// engine's own binary resolution is untouched.
func installShimDir(t *testing.T) string {
	t.Helper()

	shimDir := filepath.Join(t.TempDir(), "shim")

	err := os.MkdirAll(shimDir, 0o755)
	if err != nil {
		t.Fatalf("create shim dir: %v", err)
	}

	proxyPath := filepath.Join(shimDir, "aiproxy")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build", "-C", "../..", "-o", proxyPath, "./tests/libraries/aiproxy")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		t.Fatalf("go build aiproxy: %v", err)
	}

	for _, name := range []string{"claude", "crush", "codex"} {
		err = os.Symlink(proxyPath, filepath.Join(shimDir, name))
		if err != nil {
			t.Fatalf("install shim %s: %v", name, err)
		}
	}

	return shimDir
}

func buildTrueBDD(t *testing.T) string {
	t.Helper()

	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "true-bdd")

	// `go test` runs with cwd = the package dir (tests/bdd-cli). Build the
	// module by pointing -C up two levels to the repo root; the
	// binary entry lives under ./services/bdd-cli.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build", "-C", "../..", "-o", binPath, "./services/bdd-cli")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("go build true-bdd: %v", err)
	}

	return binPath
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

// requireConfiguredCLIs skips the suite when a CLI the engine's seed
// config binds a model tier to is not installed. Without this a
// `coder: "crush:…"` tier on a machine without crush fails deep inside
// a fix loop, minutes in, looking like a product bug.
func requireConfiguredCLIs(t *testing.T) {
	t.Helper()

	clis, err := runner.RequiredCLIs(filepath.Join("..", "..", "true-bdd", "true-bdd.yaml"))
	if err != nil {
		t.Fatalf("read engine config: %v", err)
	}

	for _, cli := range clis {
		_, lookErr := exec.LookPath(cli)
		if lookErr != nil {
			t.Skipf("engine config binds a model tier to `%s`, which is not on $PATH: %v", cli, lookErr)
		}
	}
}
