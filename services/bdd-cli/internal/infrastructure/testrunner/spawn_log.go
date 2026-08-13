package testrunner

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"
)

// Phase names for the runner invocations a walk makes, carried on every
// spawn/exit record and spliced into the captured artifact filenames.
const (
	// PhaseDiscover is the opening whole-suite run that finds failures.
	PhaseDiscover = "discover"
	// PhaseRerun is a single test re-executed after a fix was applied.
	PhaseRerun = "rerun"
	// PhaseStartupRerun is the whole-suite re-run standing in for a
	// single test when the suite never started in the first place.
	PhaseStartupRerun = "startup-rerun"
)

// spawnMeta describes one framework-runner subprocess: what is being
// executed, which framework and walk phase it belongs to, and where its
// streams are persisted. Bundled rather than passed positionally so the
// three runners cannot silently disagree about argument order.
type spawnMeta struct {
	binary    string   // log-facing command name, e.g. "npx"
	args      []string // argv after the binary
	framework string   // one of the Framework* constants
	phase     string   // one of the Phase* constants
	artifacts *Artifacts
}

// workingDir reports the directory a command with no explicit Dir will
// actually run in. Used so a spawn record never claims an empty
// directory when it means "wherever the engine was started".
func workingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	return dir
}

// effectiveDir reports where cmd will really run. An empty exec.Cmd.Dir
// means "inherit the engine's own working directory" — the go runner
// relies on that, the jest and playwright runners set Dir explicitly —
// so resolving it here keeps every log record pointing at a real path.
func effectiveDir(cmd *exec.Cmd) string {
	if cmd.Dir != "" {
		return cmd.Dir
	}

	return workingDir()
}

// runLogged executes cmd with both output streams captured, bracketed
// by the records that make a test-runner subprocess auditable: the
// exact argv before the fork, and afterwards how it ended plus where
// its output was persisted.
//
// Every framework exits non-zero on test failure, so a non-nil error
// here is routine — callers decide whether it means anything by looking
// at what landed on stdout.
func runLogged(cmd *exec.Cmd, meta spawnMeta) (bytes.Buffer, bytes.Buffer, error) {
	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	dir := effectiveDir(cmd)

	logSpawn(meta, dir)

	started := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(started)

	stdoutPath, stderrPath := meta.artifacts.Capture(
		meta.framework, meta.phase, &stdout, &stderr)

	logExit(meta, dir, elapsed, runErr, &stdout, &stderr, stdoutPath, stderrPath)

	if runErr != nil {
		return stdout, stderr, fmt.Errorf("%s exec: %w", meta.framework, runErr)
	}

	return stdout, stderr, nil
}

// logSpawn records the exact command a framework runner is about to
// execute.
//
// Discovery is otherwise invisible in the log: the engine writes nothing
// between loading the architecture and loading the checklist, so a run
// report can bound the test-run span but cannot say what ran inside it.
// The record uses the same shape the AI providers use for their own
// subprocesses ("Spawning agent CLI"), so one parser covers both.
func logSpawn(meta spawnMeta, dir string) {
	slog.Debug("Spawning test runner",
		"binary", meta.binary,
		"args", meta.args,
		"dir", dir,
		"framework", meta.framework,
		"phase", meta.phase,
	)
}

// logExit records how the subprocess ended and where its output went.
//
// logSpawn is written before the fork, so on its own it proves only
// intent: a reader cannot tell a suite that ran and failed from a
// binary that was never found. This record closes that gap with the
// facts that make the span self-evidencing — the exit code, the
// wall-clock duration, how many bytes each stream produced, and the
// files holding them — and repeats binary/args/dir so a parser can pair
// it with its spawn record without tracking position.
func logExit(
	meta spawnMeta,
	dir string,
	elapsed time.Duration,
	runErr error,
	stdout, stderr *bytes.Buffer,
	stdoutPath, stderrPath string,
) {
	code, startupErr := exitStatus(runErr)

	fields := []any{
		"binary", meta.binary,
		"args", meta.args,
		"dir", dir,
		"framework", meta.framework,
		"phase", meta.phase,
		"exit_code", code,
		"duration_ms", elapsed.Milliseconds(),
		"stdout_bytes", stdout.Len(),
		"stderr_bytes", stderr.Len(),
	}

	if stdoutPath != "" {
		fields = append(fields, "stdout_file", stdoutPath)
	}

	if stderrPath != "" {
		fields = append(fields, "stderr_file", stderrPath)
	}

	if startupErr != "" {
		fields = append(fields, "error", startupErr)
	}

	slog.Debug("Test runner returned", fields...)
}

// exitStatus classifies a cmd.Run error into the exit code the process
// reported plus, when it never reported one, the reason. A nil error is
// a clean exit 0; an *exec.ExitError carries the process's real code
// (non-zero is routine here — every framework exits non-zero on test
// failure); anything else means the process never started at all
// (binary missing, unreadable working dir), which is reported as -1 so
// it stays visibly distinct from any code a real process could return.
func exitStatus(runErr error) (int, string) {
	if runErr == nil {
		return 0, ""
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return exitErr.ExitCode(), ""
	}

	return -1, runErr.Error()
}
