//go:build bdd

package bdd_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/runner"
)

// -mode selects how fixtures reach the AI CLIs (design:
// tmp/ai-proxy-design.md). Usage:
//
//	go test -tags bdd ./tests/bdd-cli/... -mode=replay
//
// live (default) — real claude/crush, no shim, today's behavior.
// record — real CLIs behind the aiproxy shim; cassettes are (re)written
// under each selected fixture's cassettes/ dir. Filter with -run.
// replay — cassettes served by the shim; real AI CLIs not required
// (the judge still runs on real claude). A fixture without cassettes
// FAILS: silence about an un-recorded fixture would let the suite go
// green while covering less than it claims.
//
//nolint:gochecknoglobals // test-binary flag; parsed by `go test`
var proxyMode = flag.String("mode", runner.ProxyModeLive,
	"AI CLI mode for fixtures: live, record, or replay")

const (
	// fixtureTimeout caps the CLI run alone — prep and teardown have
	// their own budgets. Deliberately tight: past five minutes a
	// fixture is not slow, it is wrong — a fix prompt that cannot land,
	// or a cell whose verdict no fix can move. The engine now bounds
	// its own fix loop and fails on an applier that wrote nothing, so a
	// run that still overruns this is a bug worth failing fast on
	// rather than paying thirty minutes to confirm. A fixture whose CLI
	// invocation legitimately does heavy external work (building a
	// Docker image, re-running a browser suite) overrides it with
	// `timeout:` in its manifest.
	fixtureTimeout = 5 * time.Minute
	// judgeTimeout caps the post-run judge call. The judge gets its
	// own fresh context so it can still produce a verdict when the CLI
	// run hits fixtureTimeout (otherwise the same expired context would
	// short-circuit the judge with "context deadline exceeded" and mask
	// the real "CLI was killed" failure).
	judgeTimeout = 5 * time.Minute
)

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
		// config routes the fixture's own turns to.
		_, err := exec.LookPath("claude")
		if err != nil {
			t.Skipf("`claude` CLI not on $PATH; skipping BDD suite: %v", err)
		}

		requireConfiguredCLIs(t)
	}

	var shimDir string
	if mode != runner.ProxyModeLive {
		shimDir = installShimDir(t)
	}

	binPath := buildTrueBDD(t)

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

	fixtures, err := discoverFixtures()
	if err != nil {
		t.Fatalf("discover fixtures: %v", err)
	}

	if len(fixtures) == 0 {
		t.Fatal("no fixtures found under tests/bdd-cli/fixtures/")
	}

	names := make([]string, 0, len(fixtures))
	for _, dir := range fixtures {
		names = append(names, filepath.Base(dir))
	}

	// Written before the first fixture starts: the report's denominator
	// is what this invocation set out to run, not what it has finished.
	planned, err := runner.PlannedFixtures("TestBDDFixtures", names,
		flagValue("test.run"), flagValue("test.skip"))
	if err != nil {
		t.Fatalf("resolve planned fixtures: %v", err)
	}

	err = runner.WriteSessionMeta(sessionRoot, mode, planned)
	if err != nil {
		t.Fatalf("write session meta: %v", err)
	}

	for _, dir := range fixtures {
		name := filepath.Base(dir)

		t.Run(name, func(t *testing.T) {
			// Registered FIRST so it runs LAST: t.Cleanup is LIFO, and
			// this has to bracket every other cleanup to measure the
			// span `go test` reports. A cleanup rather than a defer
			// because it then also fires under t.Fatalf's runtime.Goexit
			// and under a panic — the two cases a statement at the end
			// of runFixture would miss, and the two a report is most
			// needed for.
			rec := runner.NewHarnessRecorder(sessionRoot, name, mode, usage)
			t.Cleanup(func() { rec.Finish(t.Failed(), t.Skipped()) })

			runFixture(t, dir, binPath, sessionRoot, judge, rec, mode, shimDir)
		})
	}
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

	cmd := exec.CommandContext(ctx, "go", "build", "-C", "../..", "-o", proxyPath, "./tests/aiproxy")
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

// proxySetup is one fixture's recording context: where the shim reads or
// writes cassettes, where its cursors live, and — in replay — the
// recorded outcome the run will be graded against.
type proxySetup struct {
	Env []string
	// Cassettes is the directory the shim uses this run: the fixture's
	// own in replay, the staging directory in record.
	Cassettes string
	StateDir  string
	// Staging is non-empty in record mode only, and is what a passing
	// run publishes.
	Staging string
	// Golden is the recorded outcome, loaded in replay mode only.
	Golden *runner.GoldenTree
}

// aiProxyEnv resolves one fixture's proxy setup.
//
// Live returns the zero value (no shim, no env — byte-for-byte today's
// run). Replay FAILS a fixture that has no cassettes or no recorded
// outcome: an un-recorded fixture is an un-run fixture, and a skip is
// the one outcome a green suite can hide. Record writes to staging and
// publishes only on a passing verdict — see promoteCassettes.
func aiProxyEnv(
	t *testing.T,
	mode, shimDir string,
	fixture *runner.Fixture,
	sessionRoot string,
) proxySetup {
	t.Helper()

	if mode == runner.ProxyModeLive {
		return proxySetup{}
	}

	setup := proxySetup{Cassettes: cassetteDir(t, fixture.Dir)}

	if mode == runner.ProxyModeReplay {
		_, statErr := os.Stat(setup.Cassettes)
		if statErr != nil {
			t.Fatalf("no cassettes for %s: %v\n%s", fixture.Name, statErr, recordHint(fixture.Name))
		}

		golden, err := runner.ReadGolden(setup.Cassettes)
		if err != nil {
			t.Fatalf("no recorded outcome for %s: %v\n%s", fixture.Name, err, recordHint(fixture.Name))
		}

		setup.Golden = golden
	}

	if mode == runner.ProxyModeRecord {
		setup.Staging = prepareStaging(t, sessionRoot, fixture.Name)
		setup.Cassettes = setup.Staging
	}

	// Cursor state lives under the run's engine-scratch tmp/ — inside
	// the snapshot window but excluded from the graded diff, and wiped
	// with the run dir.
	setup.StateDir = filepath.Join(runner.RunDir(sessionRoot, fixture.Name), "tmp", "aiproxy-state")
	setup.Env = runner.AIProxyEnv(mode, shimDir, setup.Cassettes, setup.StateDir)

	return setup
}

func recordHint(fixture string) string {
	return fmt.Sprintf("record it with: go test -tags bdd -run 'TestBDDFixtures/%s' ./tests/bdd-cli/ -mode=record",
		fixture)
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

func discoverFixtures() ([]string, error) {
	entries, err := os.ReadDir("fixtures")
	if err != nil {
		return nil, fmt.Errorf("read fixtures dir: %w", err)
	}

	var dirs []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirs = append(dirs, filepath.Join("fixtures", entry.Name()))
	}

	return dirs, nil
}

func runFixture(
	t *testing.T,
	dir, binPath, sessionRoot string,
	judge runner.Judge,
	rec *runner.HarnessRecorder,
	mode, shimDir string,
) {
	t.Helper()

	fixture, err := runner.LoadFixture(dir)
	if err != nil {
		rec.AddFailure("load fixture: " + err.Error())
		t.Fatalf("load fixture: %v", err)
	}

	// Snapshotted now, while this run's manifest is in hand. fixture.yaml
	// never reaches the tmpdir, so a report built later would otherwise
	// show today's expectations against this run's actuals.
	rec.ObserveFixture(fixture)

	proxy := aiProxyEnv(t, mode, shimDir, fixture, sessionRoot)

	timeout := fixtureTimeout
	if fixture.Timeout > 0 {
		timeout = fixture.Timeout
	}

	t.Logf("running %q (%s, timeout %s) — this can take several minutes",
		fixture.Cmd, fixture.Name, timeout)

	// The deadline is applied inside Execute, around the CLI exec alone —
	// the tmpdir build and the pre-run snapshot must not spend it.
	res, err := runner.Execute(context.Background(), fixture, binPath, sessionRoot, timeout, proxy.Env...)

	// Before the branch: a run that errored still has a diff worth
	// recording, and the tmpdir path is what makes it findable.
	rec.ObserveRun(res, err)

	if err != nil {
		rec.AddFailure("execute: " + err.Error())
		dumpRun(t, res)
		t.Fatalf("execute: %v", err)
	}

	verdict := grade(t, mode, fixture, res, judge, proxy)
	rec.ObserveVerdict(verdict)

	if verdict.Pass() {
		t.Logf("PASS %s (exit=%d, %d file change(s)) — dir: %s",
			fixture.Name, res.ExitCode, len(res.Diff), res.TmpDir)

		if proxy.Staging != "" {
			recordOutcome(t, fixture, res, proxy.Staging)
			promoteCassettes(t, fixture.Dir, proxy.Staging)
		}

		return
	}

	if proxy.Staging != "" {
		t.Logf("cassettes NOT recorded — the run failed; the rejected recording is at %s "+
			"and %s is unchanged", proxy.Staging, cassetteDir(t, fixture.Dir))
	}

	dumpRun(t, res)

	for _, msg := range verdict.Failures {
		t.Errorf("  - %s", msg)
	}

	t.Fatalf("fixture %s failed (%d check(s))", fixture.Name, len(verdict.Failures))
}

// grade picks how this run is judged. Replay compares the recording —
// deterministic, free, and strictly more specific than a rubric, since
// every AI-written file in a replayed run came out of a cassette to
// begin with. Live and record ask the judge, which is where a rubric
// earns its keep: the output is new.
func grade(
	t *testing.T,
	mode string,
	fixture *runner.Fixture,
	res *runner.RunResult,
	judge runner.Judge,
	proxy proxySetup,
) runner.Verdict {
	t.Helper()

	if mode == runner.ProxyModeReplay {
		return runner.EvaluateRecorded(fixture, res, proxy.Golden, proxy.Cassettes, proxy.StateDir)
	}

	judgeCtx, judgeCancel := context.WithTimeout(context.Background(), judgeTimeout)
	defer judgeCancel()

	return runner.Evaluate(judgeCtx, fixture, res, judge)
}

// recordOutcome writes the run's resulting tree beside its cassettes, so
// a later replay has something deterministic to be graded against.
func recordOutcome(t *testing.T, fixture *runner.Fixture, res *runner.RunResult, staging string) {
	t.Helper()

	golden := runner.NewGoldenTree(fixture.Name, res.Diff)

	err := runner.WriteGolden(staging, golden)
	if err != nil {
		t.Fatalf("write golden tree: %v", err)
	}

	t.Logf("recorded outcome: %d file(s) outside tmp/", len(golden.Files))
}

func dumpRun(t *testing.T, result *runner.RunResult) {
	t.Helper()

	if result == nil {
		return
	}

	t.Logf("tmpdir preserved at: %s", result.TmpDir)
	t.Logf("exit code: %d", result.ExitCode)

	// The clipped stderr below is a convenience; these two files hold
	// the streams in full and outlive the test process.
	if result.StdoutFile != "" {
		t.Logf("cli stdout: %s (%d bytes)", result.StdoutFile, len(result.Stdout))
	}

	if result.StderrFile != "" {
		t.Logf("cli stderr: %s (%d bytes)", result.StderrFile, len(result.Stderr))
	}

	if result.Stderr != "" {
		t.Logf("stderr (first 4KB):\n%s", clip(result.Stderr, 4096))
	}

	t.Logf("file diff (%d entries):", len(result.Diff))

	for _, change := range result.Diff {
		t.Logf("  %s %s (%d bytes)", change.Kind, change.Path, len(change.After))
	}
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return s[:n] + "…(truncated)…"
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
