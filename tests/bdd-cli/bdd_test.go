//go:build bdd

package bdd_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/runner"
)

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
	// The judge always runs on claude, regardless of which CLIs the
	// engine config routes the fixture's own turns to.
	_, err := exec.LookPath("claude")
	if err != nil {
		t.Skipf("`claude` CLI not on $PATH; skipping BDD suite: %v", err)
	}

	requireConfiguredCLIs(t)

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
			rec := runner.NewHarnessRecorder(sessionRoot, name, usage)
			t.Cleanup(func() { rec.Finish(t.Failed(), t.Skipped()) })

			runFixture(t, dir, binPath, sessionRoot, judge, rec)
		})
	}
}

func buildTrueBDD(t *testing.T) string {
	t.Helper()

	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "true-bdd")

	// `go test` runs with cwd = the package dir (tests/bdd-cli). Build the
	// module by pointing -C up two levels to the repo root; the
	// binary entry lives under ./src.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build", "-C", "../..", "-o", binPath, "./src")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("go build true-bdd: %v", err)
	}

	return binPath
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

	timeout := fixtureTimeout
	if fixture.Timeout > 0 {
		timeout = fixture.Timeout
	}

	t.Logf("running %q (%s, timeout %s) — this can take several minutes",
		fixture.Cmd, fixture.Name, timeout)

	// The deadline is applied inside Execute, around the CLI exec alone —
	// the tmpdir build and the pre-run snapshot must not spend it.
	res, err := runner.Execute(context.Background(), fixture, binPath, sessionRoot, timeout)

	// Before the branch: a run that errored still has a diff worth
	// recording, and the tmpdir path is what makes it findable.
	rec.ObserveRun(res, err)

	if err != nil {
		rec.AddFailure("execute: " + err.Error())
		dumpRun(t, res)
		t.Fatalf("execute: %v", err)
	}

	judgeCtx, judgeCancel := context.WithTimeout(context.Background(), judgeTimeout)
	defer judgeCancel()

	verdict := runner.Evaluate(judgeCtx, fixture, res, judge)
	rec.ObserveVerdict(verdict)

	if verdict.Pass() {
		t.Logf("PASS %s (exit=%d, %d file change(s)) — dir: %s",
			fixture.Name, res.ExitCode, len(res.Diff), res.TmpDir)

		return
	}

	dumpRun(t, res)

	for _, msg := range verdict.Failures {
		t.Errorf("  - %s", msg)
	}

	t.Fatalf("fixture %s failed (%d check(s))", fixture.Name, len(verdict.Failures))
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
