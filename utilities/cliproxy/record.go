package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"sync"
	"syscall"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/claude"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
	"github.com/ondatra-ai/true-bdd/pkg/console"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/fstree"
)

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

	before, err := recordingSnapshot(cwd, argv, cfg.Recording)
	if err != nil {
		return 0, fmt.Errorf("snapshot before call: %w", err)
	}

	result, err := runReal(realPath, argv)
	if err != nil {
		return 0, err
	}

	after, err := recordingSnapshot(cwd, argv, cfg.Recording)
	if err != nil {
		return 0, fmt.Errorf("snapshot after call: %w", err)
	}

	normalizeDurations := hasGoDuration(argv, result.Stdin)
	requestArgs, requestInput := canonicalGoRequest(argv, result.Stdin, normalizeDurations)
	manifest := &meta{
		NormalizeGoDurations: normalizeDurations,
		Schema:               metaSchemaVersion,
		Binary:               name,
		Argv:                 normalizedArgs(requestArgs, cwd, cfg.Recording),
		ExitCode:             result.ExitCode,
		DurationMS:           result.Duration.Milliseconds(),
		RequestHash:          hashRequest(requestArgs, requestInput, cwd, cfg.Recording),
		RecordedAt:           time.Now().UTC().Format(time.RFC3339),
	}

	dir := filepath.Join(cfg.Cassettes, cassetteName(name, idx))

	changes := normalizeChanges(fstree.Diff(before, after), cwd, cfg.Recording)

	// stdout/stderr are stored NORMALIZED (the caller parses result files by
	// their per-run tmp path, via FILE_START/FILE_END markers); sanitize runs
	// BEFORE normalize so stripped ballast leaves less noise for it to catch.
	stdout := []byte(normalize(string(sanitizeStream(result.Stdout)), cwd, cfg.Recording))
	stderr := []byte(normalize(string(result.Stderr), cwd, cfg.Recording))

	// stdin is normalized too: the fix-generator writes the NEXT turn's prompt,
	// and a model may resolve a path itself, so an absolute path reaches stdin
	// that no template contains (the Recordings gate catches it).
	stdin := []byte(normalize(string(requestInput), cwd, cfg.Recording))

	err = writeCassette(dir, manifest, stdin, stdout, stderr, changes)
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
	// the context is deliberately never cancelled here. The advisor gate is
	// set, not merely inherited — see ADR 0010.
	proc, err := spec.Start(context.Background(),
		append([]string{realPath}, argv...),
		cli.Options{Output: cli.Pipe(), Env: cli.Inherit().Set(claude.AdvisorOff)})
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", realPath, err)
	}

	stdinPipe, stdoutPipe, stderrPipe := proc.Stdin, proc.Stdout, proc.Stderr

	// Relays SIGTERM/SIGINT so an interrupted run stops the real CLI too;
	// SIGKILL needs no relay — the caller kills by process group.
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

// errKilledWhileRecording stops a recording whose real CLI a signal ended:
// the caller waits for every CLI to exit by itself, so nothing is recorded.
var errKilledWhileRecording = errors.New("the real CLI was killed by a signal while recording; nothing was recorded")

// waitExitCode reaps the child and returns its own exit code.
func waitExitCode(proc *cli.Process) (int, error) {
	result, waitErr := proc.Wait()
	if waitErr != nil {
		return 0, fmt.Errorf("wait: %w", waitErr)
	}

	if result.Signaled() {
		return 0, fmt.Errorf("%w: %s", errKilledWhileRecording, result.Signal)
	}

	return result.Code, nil
}

// Read-only calls retain their own answer while omitting volatile directories and declared paths.
func recordingSnapshot(cwd string, argv []string, rules recording) (map[string][]byte, error) {
	readOnly, answer := recordingRequest(argv, rules)
	if filepath.IsAbs(answer) {
		relative, err := filepath.Rel(cwd, answer)
		if err != nil {
			return nil, fmt.Errorf("relative answer path: %w", err)
		}

		answer = relative
	}

	values := resolvePlaceholders(cwd, rules)
	omit := func(relative string) bool {
		if rules.omits(relative) {
			return true
		}

		if !readOnly {
			return false
		}

		for _, placeholder := range rules.Placeholders {
			value := values[placeholder.Token]
			if value == "" {
				continue
			}

			directory := filepath.Join(placeholder.Directory, value)
			if strings.HasPrefix(relative, directory+string(filepath.Separator)) &&
				filepath.Clean(relative) != filepath.Clean(answer) {
				return true
			}
		}

		return false
	}

	snapshot, err := fstree.SnapshotFiltered(cwd, omit, ".git", ".DS_Store", "node_modules")
	if err != nil {
		return nil, fmt.Errorf("snapshot recording: %w", err)
	}

	return snapshot, nil
}

func recordingRequest(argv []string, rules recording) (bool, string) {
	readOnly := rules.readOnly()
	answer := ""

	for index, arg := range argv {
		if index+1 >= len(argv) {
			continue
		}

		if arg == "-s" && argv[index+1] == "read-only" {
			readOnly = true
		}

		if arg == "-o" {
			answer = argv[index+1]
		}
	}

	return readOnly, answer
}
