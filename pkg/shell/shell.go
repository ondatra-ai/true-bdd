package shell

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// Errors a caller distinguishes. A non-zero exit is none of them: that is
// Result.Code, and Result.Err turns it into an error where one is wanted.
var (
	// ErrTimeout reports that the command outlived Options.Timeout.
	ErrTimeout = errors.New("command timed out")
	// ErrNotStarted reports that the command never ran at all.
	ErrNotStarted = errors.New("command did not start")
	// ErrNotOnPath reports a binary absent from PATH.
	ErrNotOnPath = errors.New("not on PATH")
)

// labelWords is how much of an argv a diagnostic names: enough to identify
// the command, short enough not to paste a whole prompt into an error.
const labelWords = 4

// Find resolves a binary on PATH, for the caller that needs the path itself
// rather than the yes-or-no: the claude transport spawns it.
func Find(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%w: %s: %w", ErrNotOnPath, name, err)
	}

	return path, nil
}

// Require reports the first named binary missing from PATH, for the preflight
// checks that would rather refuse up front than fail mid-run.
func Require(names ...string) error {
	for _, name := range names {
		_, err := Find(name)
		if err != nil {
			return err
		}
	}

	return nil
}

// Run spawns argv, waits for it, and reports what happened. A non-zero exit
// is Result.Code and not an error: the error is reserved for a command that
// did not run to completion, which is a timeout or a failure to start.
func Run(ctx context.Context, argv []string, opt Options) (Result, error) {
	if opt.Timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, opt.Timeout)
		defer cancel()
	}

	cmd, err := newCommand(ctx, argv, opt)
	if err != nil {
		return Result{Code: NotStarted}, err
	}

	stdout, stderr := opt.Output.wire(cmd)

	started := time.Now()

	logSpawn(argv, cmd.Dir)
	runErr := cmd.Run()

	if ctx.Err() != nil {
		return Result{Code: NotStarted},
			fmt.Errorf("%w after %s: %s", ErrTimeout, opt.Timeout, label(argv))
	}

	code, signal, ran := classify(runErr)
	result := Result{
		Stdout: stdout.String(), Stderr: stderr.String(), Code: code, Signal: signal,
	}

	logExit(argv, result, time.Since(started))

	if !ran {
		return result, fmt.Errorf("%w: %s: %w", ErrNotStarted, label(argv), runErr)
	}

	return result, nil
}

// newCommand builds the child from argv and opt, short of its streams, which
// Run and Start attach differently.
func newCommand(ctx context.Context, argv []string, opt Options) (*exec.Cmd, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("%w: empty argv", ErrNotStarted)
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = opt.Dir
	cmd.Env = opt.Env.build()
	cmd.Stdin = opt.Stdin
	cmd.ExtraFiles = opt.ExtraFiles
	cmd.WaitDelay = opt.WaitDelay

	if opt.Group {
		applyGroup(cmd)
	}

	return cmd, nil
}

// applyGroup makes the child a process group leader and cancels it by killing
// the group, so a cancelled context takes its grandchildren with it.
func applyGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}

		return nil
	}
}

// logSpawn records the argv before the fork. Debug, so it reaches the JSON
// log a report is folded out of without touching the terminal.
func logSpawn(argv []string, dir string) {
	slog.Debug("Spawning subprocess", "binary", argv[0], "args", argv[1:], "dir", dir)
}

// logExit pairs logSpawn: intent alone cannot tell "ran and failed" from
// "never found", so the exit facts are recorded too. NOT `duration_ms` —
// scripts/report reads that key as "I am a report leaf" (record.go:31).
func logExit(argv []string, result Result, elapsed time.Duration) {
	slog.Debug("Subprocess returned",
		"binary", argv[0],
		"args", argv[1:],
		"exit_code", result.Code,
		"signal", result.Signal.String(),
		"elapsed_ms", elapsed.Milliseconds(),
		"stdout_bytes", len(result.Stdout),
		"stderr_bytes", len(result.Stderr),
	)
}

// label names a command in a diagnostic.
func label(argv []string) string {
	return strings.Join(argv[:min(labelWords, len(argv))], " ")
}
