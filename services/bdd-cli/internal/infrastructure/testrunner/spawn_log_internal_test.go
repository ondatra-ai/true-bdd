package testrunner

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestExitStatusClassification pins the distinction the exit record
// exists to make: a framework that ran and failed must be reported with
// the code it returned, while a process that never started must be
// reported as -1 with a reason. Collapsing the two is what made a
// failed run indistinguishable from a run that never happened.
func TestExitStatusClassification(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	ranAndFailed := exec.CommandContext(ctx, "sh", "-c", "exit 3").Run()
	neverStarted := exec.CommandContext(ctx, "true-bdd-no-such-binary").Run()

	tests := []struct {
		name       string
		err        error
		wantCode   int
		wantReason bool
	}{
		{name: "clean exit", err: nil, wantCode: 0},
		{name: "ran and failed", err: ranAndFailed, wantCode: 3},
		{name: "wrapped by the caller", err: fmt.Errorf("playwright exec: %w", ranAndFailed), wantCode: 3},
		{name: "never started", err: neverStarted, wantCode: -1, wantReason: true},
		{name: "not an exec error", err: errors.New("boom"), wantCode: -1, wantReason: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			code, reason := exitStatus(test.err)
			if code != test.wantCode {
				t.Errorf("exit code = %d, want %d", code, test.wantCode)
			}

			if (reason != "") != test.wantReason {
				t.Errorf("reason = %q, want present=%v", reason, test.wantReason)
			}
		})
	}
}

// TestRunLoggedCapturesBothStreams checks that routing every runner
// through runLogged did not change what the callers receive: they
// branch on stdout being empty, so a swapped or dropped stream would
// silently turn a real report into a discovery error. It also pins the
// on-disk capture, which is the whole point of the detour — a run whose
// output only ever existed in memory left nothing to audit.
func TestRunLoggedCapturesBothStreams(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cmd := exec.CommandContext(t.Context(), "sh", "-c", "echo out; echo err >&2; exit 1")

	stdout, stderr, err := runLogged(cmd, spawnMeta{
		binary:    "sh",
		args:      []string{"-c", "..."},
		framework: FrameworkPlaywright,
		phase:     PhaseDiscover,
		artifacts: NewArtifacts(dir),
	})
	if err == nil {
		t.Fatal("want non-nil error for a non-zero exit")
	}

	if got := stdout.String(); got != "out\n" {
		t.Errorf("stdout = %q, want %q", got, "out\n")
	}

	if got := stderr.String(); got != "err\n" {
		t.Errorf("stderr = %q, want %q", got, "err\n")
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("wrapped error lost its *exec.ExitError: %v", err)
	}

	assertFile(t, filepath.Join(dir, "testrun-001-playwright-discover-stdout.json"), "out\n")
	assertFile(t, filepath.Join(dir, "testrun-001-playwright-discover-stderr.txt"), "err\n")
}

// TestArtifactsCaptureSequencesAcrossFrameworks pins the shared counter:
// one writer serves all three runners, so the numbers have to order a
// run's spawns against each other rather than restarting per framework.
// It also covers the empty-stream case — a zero-byte file is the
// evidence that a framework printed nothing there.
func TestArtifactsCaptureSequencesAcrossFrameworks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	artifacts := NewArtifacts(dir)

	var empty bytes.Buffer

	first, _ := artifacts.Capture(FrameworkPlaywright, PhaseDiscover, &empty, &empty)
	second, _ := artifacts.Capture(FrameworkGoTest, PhaseRerun, &empty, &empty)

	if want := filepath.Join(dir, "testrun-001-playwright-discover-stdout.json"); first != want {
		t.Errorf("first capture = %q, want %q", first, want)
	}

	if want := filepath.Join(dir, "testrun-002-go-test-rerun-stdout.jsonl"); second != want {
		t.Errorf("second capture = %q, want %q", second, want)
	}

	assertFile(t, first, "")
}

// TestArtifactsNilCapturesNothing covers a runner built without a run
// directory: capture is optional plumbing and must never panic.
func TestArtifactsNilCapturesNothing(t *testing.T) {
	t.Parallel()

	var (
		nilArtifacts *Artifacts
		empty        bytes.Buffer
	)

	stdoutPath, stderrPath := nilArtifacts.Capture(
		FrameworkJest, PhaseDiscover, &empty, &empty)
	if stdoutPath != "" || stderrPath != "" {
		t.Errorf("nil writer returned paths: %q / %q", stdoutPath, stderrPath)
	}
}

// assertFile checks a captured artifact exists with the expected body.
func assertFile(t *testing.T, path, want string) {
	t.Helper()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	if string(got) != want {
		t.Errorf("%s = %q, want %q", filepath.Base(path), got, want)
	}
}
