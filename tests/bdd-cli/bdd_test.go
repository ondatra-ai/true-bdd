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
	// fixtureTimeout caps the CLI run alone. The `--fix` fixtures make
	// 30+ sequential Claude calls (walk → fix loop → re-walk) and were
	// previously killed at the 15-minute mark with the re-walk still
	// pending.
	fixtureTimeout = 30 * time.Minute
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

	fixtures, err := discoverFixtures()
	if err != nil {
		t.Fatalf("discover fixtures: %v", err)
	}

	if len(fixtures) == 0 {
		t.Fatal("no fixtures found under tests/bdd-cli/fixtures/")
	}

	for _, dir := range fixtures {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			runFixture(t, dir, binPath, sessionRoot, judge)
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

func runFixture(t *testing.T, dir, binPath, sessionRoot string, judge runner.Judge) {
	t.Helper()

	fixture, err := runner.LoadFixture(dir)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	runCtx, runCancel := context.WithTimeout(context.Background(), fixtureTimeout)
	defer runCancel()

	t.Logf("running %q (%s) — this can take several minutes", fixture.Cmd, fixture.Name)

	res, err := runner.Execute(runCtx, fixture, binPath, sessionRoot)
	if err != nil {
		dumpRun(t, res)
		t.Fatalf("execute: %v", err)
	}

	judgeCtx, judgeCancel := context.WithTimeout(context.Background(), judgeTimeout)
	defer judgeCancel()

	verdict := runner.Evaluate(judgeCtx, fixture, res, judge)

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
