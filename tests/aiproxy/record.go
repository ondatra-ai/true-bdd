package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/ondatra-ai/true-bdd/tests/internal/fstree"
)

// snapshotSkipDirs are workdir subtrees excluded from per-call
// snapshots for cost and noise. tmp/ is deliberately NOT here: the
// validation and apply prompts tell the AI to WRITE its result files
// under tmp/<run-dir>/, and the engine reads them back to build the
// next prompt — replay must reproduce them or the conversation
// diverges. Engine-owned tmp/ paths are filtered per change instead
// (isEngineOwned).
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

	// stdout/stderr are stored NORMALIZED: the engine parses result
	// files out of the response via FILE_START/FILE_END markers that
	// embed the per-run tmp path (checklist_evaluator.parseResultFile),
	// so replayed output must carry the REPLAY run's paths, not the
	// recording's. Replay reverses the substitution before emitting.
	// Sanitized BEFORE normalizing: the ballast fields are what carry
	// the recording machine's home paths, so dropping them first leaves
	// the placeholder substitution with less to catch — and what it does
	// catch is a genuine path the engine may need mapped, not an
	// inventory nobody reads.
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
	// Transparent proxy: argv passes through; lifetime is the parent's.
	cmd := exec.Command(realPath, argv...) //nolint:gosec,noctx

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	start := time.Now()

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", realPath, err)
	}

	forwardSignals(cmd)

	var stdinBuf, stdoutBuf, stderrBuf lockedBuffer

	// The stdin pump is NOT waited on: the parent may hold our stdin
	// open past the child's exit, and the copy would block forever.
	// lockedBuffer makes reading its partial capture safe.
	go func() {
		_, _ = io.Copy(io.MultiWriter(stdinPipe, &stdinBuf), os.Stdin)
		_ = stdinPipe.Close()
	}()

	outPumps := []struct {
		dst io.Writer
		src io.Reader
	}{
		{io.MultiWriter(os.Stdout, &stdoutBuf), stdoutPipe},
		{io.MultiWriter(os.Stderr, &stderrBuf), stderrPipe},
	}

	var pumps sync.WaitGroup

	pumps.Add(len(outPumps))

	for _, p := range outPumps {
		go pump(&pumps, p.dst, p.src)
	}

	pumps.Wait()

	code, err := waitExitCode(cmd)
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

// waitExitCode reaps the child. A signal-killed child reports exit code
// 0: the engine's shutdown paths (claude's stdin-close → SIGTERM,
// crush's group SIGKILL) fire after it has consumed the output it
// needed, so the recorded turn was a success from the engine's view.
func waitExitCode(cmd *exec.Cmd) (int, error) {
	waitErr := cmd.Wait()
	if waitErr == nil {
		return 0, nil
	}

	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) {
		return 0, fmt.Errorf("wait: %w", waitErr)
	}

	code := exitErr.ExitCode()
	if code < 0 {
		code = 0
	}

	return code, nil
}

// forwardSignals relays SIGTERM/SIGINT to the child. The claude
// transport's shutdown is stdin-close → SIGTERM → 5s → SIGKILL
// (claudecode/internal/subprocess/transport.go): relaying keeps the
// child on that schedule while the shim finalizes the cassette inside
// the grace window. SIGKILL is not relayable — but the engine only
// SIGKILLs by process group (cli_invocation.go), which takes the child
// down with the shim.
func forwardSignals(cmd *exec.Cmd) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		for sig := range sigs {
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}()
}
