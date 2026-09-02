package steps

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
	"github.com/ondatra-ai/true-bdd/pkg/console"
)

// ErrNoRemote is returned when a step asserts on a session and no Given
// step started a remote.
var ErrNoRemote = errors.New("no Given step started a remote")

// ErrUnknownSignal is returned when a step names a signal this suite does
// not send.
var ErrUnknownSignal = errors.New("unknown signal")

// Remote is one `true-bdd remote` connected to the relay under test: the
// host-folder agent whose registration the session clauses are about.
type Remote struct {
	// Dir is the directory it was STARTED in — a symlink when the scenario
	// started it through one, which is what makes the folder clause mean
	// something.
	Dir string
	// PID identifies its entry in the registry: a folder may carry more
	// than one session, and the session id is the remote's to choose.
	PID int

	process *cli.Process
}

// startRemote spawns the remote in dir and returns without waiting. PWD
// travels along, so a remote reading the environment rather than the
// kernel sees the path it was handed; the child outlives this call.
func startRemote(harness *Harness, dir string) (*Remote, error) {
	binary, err := harness.CLIBinary()
	if err != nil {
		return nil, err
	}

	process, err := spec.Start(context.Background(),
		[]string{binary, "remote", "--server", harness.BaseURL},
		cli.Options{
			Dir:    dir,
			Env:    cli.Inherit().Set("PWD=" + dir),
			Output: cli.Streams(console.Err(), console.Err()),
		})
	if err != nil {
		return nil, fmt.Errorf("start a remote in %s: %w", dir, err)
	}

	return &Remote{Dir: dir, PID: process.Pid(), process: process}, nil
}

// stop ends the remote. Kill rather than a polite signal: the scenario is
// over, and a remote left running holds its folder and its session.
func (remote *Remote) stop() {
	_ = remote.process.Signal(os.Kill)
	_, _ = remote.process.Wait()
}

// signal delivers one signal to the remote process itself. pkg/shell starts
// it in its own process group, so a frozen remote freezes exactly as the
// CLI would, children untouched.
func (remote *Remote) signal(sig os.Signal) error {
	err := remote.process.Signal(sig)
	if err != nil {
		return fmt.Errorf("signal the remote (pid %d): %w", remote.PID, err)
	}

	return nil
}

// signalNamed maps the name a scenario writes onto the signal it means.
func signalNamed(name string) (os.Signal, error) {
	switch name {
	case "SIGSTOP":
		return syscall.SIGSTOP, nil
	case "SIGCONT":
		return syscall.SIGCONT, nil
	case "SIGINT":
		return syscall.SIGINT, nil
	case "SIGTERM":
		return syscall.SIGTERM, nil
	case "SIGKILL":
		return syscall.SIGKILL, nil
	default:
		return nil, fmt.Errorf("%w: %q (SIGSTOP, SIGCONT, SIGINT, SIGTERM or SIGKILL)",
			ErrUnknownSignal, name)
	}
}
