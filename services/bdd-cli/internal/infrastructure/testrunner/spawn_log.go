package testrunner

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
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

// spawnMeta describes one framework-runner subprocess: what's executed,
// which framework/phase it belongs to, and where its streams persist.
// Bundled rather than positional so same-typed args can't silently swap.
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

// effectiveDir reports where the command will really run. An empty Dir means
// "inherit the engine's cwd" (what the go runner relies on; jest and
// playwright set it) — resolved here so every record points at a real path.
func effectiveDir(dir string) string {
	if dir != "" {
		return dir
	}

	return workingDir()
}

// runLogged runs the suite's argv with both streams captured, bracketed by
// records that make it auditable: the argv before the fork, how it ended after.
// Every framework exits non-zero on test failure, so a non-nil error is routine.
func runLogged(
	ctx context.Context, argv []string, dir string, meta spawnMeta,
) (bytes.Buffer, bytes.Buffer, error) {
	where := effectiveDir(dir)

	logSpawn(meta, where)

	started := time.Now()
	result, runErr := spec.Run(ctx, argv, cli.Options{Dir: dir})
	elapsed := time.Since(started)

	var stdout, stderr bytes.Buffer

	stdout.WriteString(result.Stdout)
	stderr.WriteString(result.Stderr)

	stdoutPath, stderrPath := meta.artifacts.Capture(
		meta.framework, meta.phase, &stdout, &stderr)

	logExit(meta, where, elapsed, result, runErr, &stdout, &stderr, stdoutPath, stderrPath)

	if runErr != nil {
		return stdout, stderr, fmt.Errorf("%s exec: %w", meta.framework, runErr)
	}

	if result.Code != 0 {
		return stdout, stderr, fmt.Errorf("%s exec: %w", meta.framework, result.Err())
	}

	return stdout, stderr, nil
}

// logSpawn records the exact command about to execute — discovery is
// otherwise invisible in the log, so a run report couldn't say what ran.
// Same record shape as the AI providers' "Spawning agent CLI" so one parser covers both.
func logSpawn(meta spawnMeta, dir string) {
	slog.Debug("Spawning test runner",
		"binary", meta.binary,
		"args", meta.args,
		"dir", dir,
		"framework", meta.framework,
		"phase", meta.phase,
	)
}

// logExit records how the subprocess ended: logSpawn alone proves only
// intent (a reader can't tell "ran and failed" from "binary never found"),
// so this repeats binary/args/dir plus the exit facts, letting a parser pair the two without tracking position.
func logExit(
	meta spawnMeta,
	dir string,
	elapsed time.Duration,
	result cli.Result,
	runErr error,
	stdout, stderr *bytes.Buffer,
	stdoutPath, stderrPath string,
) {
	code, startupErr := result.Code, ""
	if runErr != nil {
		startupErr = runErr.Error()
	}

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
