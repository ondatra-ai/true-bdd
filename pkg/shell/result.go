package shell

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
)

// ErrExit reports a non-zero exit. Run never returns it: an exit code is an
// answer, and the three predecessors disagreed about whether it was a failure.
// Result.Err wraps it for the callers that want one.
var ErrExit = errors.New("command failed")

// NotStarted is Code when the process never ran. A signalled child also
// reports -1, because that is what WaitStatus.ExitStatus returns for one;
// Signaled and Run's error are what tell the two apart.
const NotStarted = -1

// Result is one finished command. Stdout and Stderr hold what was captured,
// which is empty when Options.Output sent the bytes somewhere else.
type Result struct {
	Stdout string
	Stderr string
	// Code is the child's exit status, or NotStarted.
	Code int
	// Signal is what killed the child, and is zero when nothing did. The
	// number, not its name: remote/supervisor re-raises it on itself to
	// propagate the death faithfully, which a name cannot express.
	Signal syscall.Signal
}

// Signaled reports whether a signal ended the child rather than a return.
func (r Result) Signaled() bool { return r.Signal != 0 }

// Err reports a non-zero exit as an error. The diagnostic prefers stderr and
// falls back to stdout, because a tool that failed usually says why on stderr
// and a few say it on stdout. Callers truncate; this does not.
func (r Result) Err() error {
	if r.Code == 0 {
		return nil
	}

	return fmt.Errorf("%w (%d): %s", ErrExit, r.Code, firstNonEmpty(r.Stderr, r.Stdout))
}

// classify reports an exit status, the signal that produced it, and whether
// the child ran at all. ExitCode() alone loses the signal, which is the one
// fact remote/terminal_envelope exists to report.
func classify(runErr error) (int, syscall.Signal, bool) {
	if runErr == nil {
		return 0, 0, true
	}

	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) {
		return NotStarted, 0, false
	}

	// A signalled child's ExitStatus is -1, the same number as NotStarted,
	// so `started` and not the code is what separates them.
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if ok && status.Signaled() {
		return status.ExitStatus(), status.Signal(), true
	}

	return exitErr.ExitCode(), 0, true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}
