package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

var (
	errCassetteMissing = errors.New("no cassette for this call — the engine made more AI calls than were " +
		"recorded; re-record with `go test -tags bdd ./tests/bdd-cli/... -mode=record`")
	errCassetteStale = errors.New("stale cassette — the request differs from the recording (template, " +
		"checklist, or engine change); re-record with `go test -tags bdd ./tests/bdd-cli/... -mode=record`")
)

// replay serves one recorded call: verify the incoming request against
// the cassette, apply the recorded file effects, emit the recorded
// output byte-for-byte, exit with the recorded code. The real CLI is
// never touched, and there is deliberately NO fall-through to it —
// VCR "record_mode=none" semantics.
func replay(cfg config, name string, argv []string) (int, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return 0, fmt.Errorf("getwd: %w", err)
	}

	idx, err := nextCallIndex(cfg.StateDir, name)
	if err != nil {
		return 0, err
	}

	dir := filepath.Join(cfg.Cassettes, cassetteName(name, idx))

	manifest, err := readMeta(dir)
	if err != nil {
		return 0, fmt.Errorf("%w (%s: %w)", errCassetteMissing, dir, err)
	}

	recordedStdin, err := os.ReadFile(filepath.Join(dir, stdinFile))
	if err != nil {
		return 0, fmt.Errorf("read recorded stdin: %w", err)
	}

	stdin := startStdinCollector()

	incoming := stdin.collect(len(recordedStdin))

	if requestHash(argv, incoming, cwd) != manifest.RequestHash {
		return 0, fmt.Errorf("%w (%s)", errCassetteStale, dir)
	}

	// File effects FIRST: the engine re-reads the touched files the
	// moment the turn's output completes.
	err = applyFSDiff(dir, manifest, cwd)
	if err != nil {
		return 0, err
	}

	err = emitStreams(dir, cwd)
	if err != nil {
		return 0, err
	}

	// Stay alive, like the real CLI, until the engine closes stdin —
	// see awaitClose for why exiting now would truncate the response.
	stdin.awaitClose()

	return manifest.ExitCode, nil
}

// applyFSDiff reproduces the recorded call's file effects in cwd.
// Cassette paths and contents are stored normalized ({{CWD}},
// {{RUN_DIR}}); both are mapped back onto the live run before writing —
// the result files the AI writes under tmp/<run-dir>/ must land in
// THIS run's directory for the engine to read them back.
func applyFSDiff(dir string, manifest *meta, cwd string) error {
	runDir := findCurrentRunDir(cwd)

	for _, rel := range slices.Concat(manifest.FSDiff.Created, manifest.FSDiff.Modified) {
		err := applyOneWrite(dir, rel, cwd, runDir)
		if err != nil {
			return err
		}
	}

	for _, rel := range manifest.FSDiff.Deleted {
		livePath, err := denormalize(rel, cwd, runDir)
		if err != nil {
			return fmt.Errorf("map fsdiff path %s: %w", rel, err)
		}

		err = os.Remove(filepath.Join(cwd, livePath))
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("apply fsdiff delete %s: %w", livePath, err)
		}
	}

	return nil
}

// applyOneWrite materializes one created/modified cassette entry at its
// live location.
func applyOneWrite(dir, rel, cwd, runDir string) error {
	data, err := os.ReadFile(filepath.Join(dir, fsdiffDir, afterDir, rel))
	if err != nil {
		return fmt.Errorf("read fsdiff for %s: %w", rel, err)
	}

	livePath, err := denormalize(rel, cwd, runDir)
	if err != nil {
		return fmt.Errorf("map fsdiff path %s: %w", rel, err)
	}

	liveContent, err := denormalize(string(data), cwd, runDir)
	if err != nil {
		return fmt.Errorf("map fsdiff content %s: %w", rel, err)
	}

	target := filepath.Join(cwd, livePath)

	err = os.MkdirAll(filepath.Dir(target), dirPerm)
	if err != nil {
		return fmt.Errorf("create dir for %s: %w", livePath, err)
	}

	err = os.WriteFile(target, []byte(liveContent), filePerm)
	if err != nil {
		return fmt.Errorf("apply fsdiff %s: %w", livePath, err)
	}

	return nil
}

// emitStreams replays the recorded stdout (and stderr, when present),
// denormalized onto the live run: the FILE_START/FILE_END markers the
// engine parses out of the response must name THIS run's tmp paths.
func emitStreams(dir, cwd string) error {
	runDir := findCurrentRunDir(cwd)

	stdout, err := os.ReadFile(filepath.Join(dir, stdoutFile))
	if err != nil {
		return fmt.Errorf("read recorded stdout: %w", err)
	}

	liveStdout, err := denormalize(string(stdout), cwd, runDir)
	if err != nil {
		return fmt.Errorf("map recorded stdout: %w", err)
	}

	_, err = os.Stdout.WriteString(liveStdout)
	if err != nil {
		return fmt.Errorf("emit stdout: %w", err)
	}

	stderrBytes, readErr := os.ReadFile(filepath.Join(dir, stderrFile))
	if readErr == nil {
		liveStderr, mapErr := denormalize(string(stderrBytes), cwd, runDir)
		if mapErr != nil {
			return fmt.Errorf("map recorded stderr: %w", mapErr)
		}

		_, _ = os.Stderr.WriteString(liveStderr)
	}

	return nil
}
