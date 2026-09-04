package shell

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"time"
)

// Process is a started command the caller drives itself. It exists for the
// six sites Run cannot express: a bidirectional JSON protocol, supervised
// process groups, a byte-exact stdio proxy, and a long-lived server.
type Process struct {
	// Stdin, Stdout and Stderr are the pipes Options.Output asked for, and
	// are nil for a stream the sink sent somewhere else.
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	Stderr io.ReadCloser

	cmd     *exec.Cmd
	argv    []string
	stdout  *bytes.Buffer
	stderr  *bytes.Buffer
	started time.Time
}

// Start spawns argv and returns without waiting. The caller owns the
// lifetime: nothing here kills the child, and Wait must be called exactly
// once or the process is left a zombie.
func Start(ctx context.Context, argv []string, opt Options) (*Process, error) {
	cmd, err := newCommand(ctx, argv, opt)
	if err != nil {
		return nil, err
	}

	stdout, stderr := opt.Output.wire(cmd)

	process := &Process{cmd: cmd, argv: argv, stdout: stdout, stderr: stderr}

	err = process.openPipes(opt)
	if err != nil {
		return nil, err
	}

	logSpawn(argv, cmd.Dir)

	process.started = time.Now()

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrNotStarted, label(argv), err)
	}

	return process, nil
}

// Wait blocks until the child ends and reports what happened, on the same
// terms as Run: a non-zero exit is Result.Code, not an error.
func (p *Process) Wait() (Result, error) {
	waitErr := p.cmd.Wait()
	code, signalName, ran := classify(waitErr)

	result := Result{
		Stdout: p.stdout.String(),
		Stderr: p.stderr.String(),
		Code:   code,
		Signal: signalName,
	}

	logExit(p.argv, result, time.Since(p.started))

	if !ran {
		return result, fmt.Errorf("%w: %s: %w", ErrNotStarted, label(p.argv), waitErr)
	}

	return result, nil
}

// Signal sends one signal to the child.
func (p *Process) Signal(sig os.Signal) error {
	if p.cmd.Process == nil {
		return fmt.Errorf("%w: %s", ErrNotStarted, label(p.argv))
	}

	err := p.cmd.Process.Signal(sig)
	if err != nil {
		return fmt.Errorf("signal %s to %s: %w", sig, label(p.argv), err)
	}

	return nil
}

// Pid is the child's process id, or zero before it started.
func (p *Process) Pid() int {
	if p.cmd.Process == nil {
		return 0
	}

	return p.cmd.Process.Pid
}

// ForwardSignals relays the named signals to the child until the returned
// stop is called. pkg/testkit/aiproxy stands in for the real CLI, so a
// SIGTERM meant for that CLI has to arrive there.
func (p *Process) ForwardSignals(signals ...os.Signal) func() {
	inbox := make(chan os.Signal, 1)
	signal.Notify(inbox, signals...)

	go func() {
		for received := range inbox {
			_ = p.Signal(received)
		}
	}()

	return func() {
		signal.Stop(inbox)
		close(inbox)
	}
}

// openPipes attaches a pipe to every stream the sink left unset. A nil
// writer means "hand it to me", which is what lets the claude transport take
// stdout as a pipe while its stderr goes to a real file.
func (p *Process) openPipes(opt Options) error {
	var err error

	if opt.Stdin == nil {
		p.Stdin, err = p.cmd.StdinPipe()
		if err != nil {
			return fmt.Errorf("stdin pipe for %s: %w", label(p.argv), err)
		}
	}

	if p.cmd.Stdout == nil {
		p.Stdout, err = p.cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("stdout pipe for %s: %w", label(p.argv), err)
		}
	}

	if p.cmd.Stderr == nil {
		p.Stderr, err = p.cmd.StderrPipe()
		if err != nil {
			return fmt.Errorf("stderr pipe for %s: %w", label(p.argv), err)
		}
	}

	return nil
}
