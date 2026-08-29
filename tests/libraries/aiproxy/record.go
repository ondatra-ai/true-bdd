package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
	"github.com/ondatra-ai/true-bdd/pkg/console"
	"github.com/ondatra-ai/true-bdd/tests/libraries/fstree"
)

// snapshotSkipDirs are workdir subtrees excluded from per-call snapshots.
// tmp/ deliberately stays IN: the AI writes result files there for the
// engine to read back, so replay must reproduce them (isEngineOwned filters engine-owned tmp/ paths separately).
func snapshotSkipDirs() []string {
	return []string{".crush", ".git", "node_modules"}
}

// record runs the real CLI as a transparent proxy and persists the call
// as a cassette: argv, stdin, the byte-exact output streams, exit code,
// and the working-tree diff the call produced.
func record(cfg config, name string, argv []string) (int, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return 0, fmt.Errorf("getwd: %w", err)
	}

	idx, err := nextCallIndex(cfg.StateDir, name)
	if err != nil {
		return 0, err
	}

	realPath, err := resolveRealBinary(name, cfg.ShimDir)
	if err != nil {
		return 0, err
	}

	before, err := fstree.Snapshot(cwd, snapshotSkipDirs()...)
	if err != nil {
		return 0, fmt.Errorf("snapshot before call: %w", err)
	}

	result, err := runReal(realPath, argv)
	if err != nil {
		return 0, err
	}

	after, err := fstree.Snapshot(cwd, snapshotSkipDirs()...)
	if err != nil {
		return 0, fmt.Errorf("snapshot after call: %w", err)
	}

	manifest := &meta{
		Schema:      metaSchemaVersion,
		Binary:      name,
		Argv:        normalizeArgv(argv, cwd),
		ExitCode:    result.ExitCode,
		DurationMS:  result.Duration.Milliseconds(),
		RequestHash: requestHash(argv, result.Stdin, cwd),
		RecordedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	dir := filepath.Join(cfg.Cassettes, cassetteName(name, idx))

	changes := normalizeChanges(fstree.Diff(before, after), cwd)

	// stdout/stderr are stored NORMALIZED (the engine parses result files by
	// their per-run tmp path, via FILE_START/FILE_END markers); sanitize runs
	// BEFORE normalize so stripped ballast leaves less noise for it to catch.
	stdout := []byte(normalizeText(string(sanitizeStream(result.Stdout)), cwd))
	stderr := []byte(normalizeText(string(result.Stderr), cwd))

	err = writeCassette(dir, manifest, result.Stdin, stdout, stderr, changes)
	if err != nil {
		return 0, err
	}

	return result.ExitCode, nil
}

// realRun captures everything one passthrough execution produced.
type realRun struct {
	Stdin    []byte
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Duration time.Duration
}

// runReal spawns the real CLI with identical argv/env/cwd and pumps all
// three streams through unbuffered — claude's stream-json conversation
// is frame-by-frame interactive, so any buffering would deadlock it.
func runReal(realPath string, argv []string) (*realRun, error) {
	start := time.Now()

	// Transparent proxy: argv passes through; lifetime is the parent's, so
	// the context is deliberately never cancelled here.
	proc, err := spec.Start(context.Background(),
		append([]string{realPath}, argv...), cli.Options{Output: cli.Pipe()})
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", realPath, err)
	}

	stdinPipe, stdoutPipe, stderrPipe := proc.Stdin, proc.Stdout, proc.Stderr

	// Relays SIGTERM/SIGINT, keeping the child on claude's
	// stdin-close→SIGTERM→5s→SIGKILL schedule (transport.go) while the shim
	// finalizes; SIGKILL needs no relay — the engine kills by process group.
	defer proc.ForwardSignals(syscall.SIGTERM, syscall.SIGINT)()

	var stdinBuf, stdoutBuf, stderrBuf lockedBuffer

	// The stdin pump is NOT waited on: the parent may hold our stdin
	// open past the child's exit, and the copy would block forever.
	// lockedBuffer makes reading its partial capture safe.
	go func() {
		_, _ = io.Copy(io.MultiWriter(stdinPipe, &stdinBuf), console.In())
		_ = stdinPipe.Close()
	}()

	outPumps := []struct {
		dst io.Writer
		src io.Reader
	}{
		{io.MultiWriter(console.Out(), &stdoutBuf), stdoutPipe},
		{io.MultiWriter(console.Err(), &stderrBuf), stderrPipe},
	}

	var pumps sync.WaitGroup

	pumps.Add(len(outPumps))

	for _, p := range outPumps {
		go pump(&pumps, p.dst, p.src)
	}

	pumps.Wait()

	code, err := waitExitCode(proc)
	if err != nil {
		return nil, err
	}

	return &realRun{
		Stdin:    stdinBuf.Bytes(),
		Stdout:   stdoutBuf.Bytes(),
		Stderr:   stderrBuf.Bytes(),
		ExitCode: code,
		Duration: time.Since(start),
	}, nil
}

func pump(group *sync.WaitGroup, dst io.Writer, src io.Reader) {
	defer group.Done()

	_, _ = io.Copy(dst, src)
}

// waitExitCode reaps the child. A signal-killed child reports exit code 0:
// the engine's shutdown signals fire only after it has consumed the output
// it needed, so the recorded turn was a success from the engine's view.
func waitExitCode(proc *cli.Process) (int, error) {
	result, waitErr := proc.Wait()
	if waitErr != nil {
		return 0, fmt.Errorf("wait: %w", waitErr)
	}

	code := result.Code
	if code < 0 {
		code = 0
	}

	return code, nil
}
