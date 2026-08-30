package testrunner

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// TestRunLoggedCapturesBothStreams pins that runLogged doesn't swap or drop
// stdout/stderr — callers branch on stdout being empty — and that both
// land on disk as the audit trail.
func TestRunLoggedCapturesBothStreams(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	argv := []string{"sh", "-c", "echo out; echo err >&2; exit 1"}

	stdout, stderr, err := runLogged(argv, "", spawnMeta{
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

	if !errors.Is(err, cli.ErrExit) {
		t.Errorf("wrapped error lost the non-zero exit: %v", err)
	}

	assertFile(t, filepath.Join(dir, "testrun-001-playwright-discover-stdout.json"), "out\n")
	assertFile(t, filepath.Join(dir, "testrun-001-playwright-discover-stderr.txt"), "err\n")
}

// TestArtifactsCaptureSequencesAcrossFrameworks pins the shared counter: one
// writer serves all three runners, so sequence numbers order a run's spawns
// instead of restarting per framework; it also covers the empty-stream case.
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

	got, err := disk.Read(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	if string(got) != want {
		t.Errorf("%s = %q, want %q", filepath.Base(path), got, want)
	}
}
