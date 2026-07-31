package remote

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Production shutdown escalation bounds (plan §3.2): forward SIGINT + close
// stdin, wait, then SIGTERM the group, wait, then SIGKILL the group. A
// healthy claude dies on the SIGINT + stdin EOF; the escalation is forced
// only for a child that ignores the softer signals.
const (
	escalateSIGINTWait  = 20 * time.Second
	escalateSIGTERMWait = 5 * time.Second
)

// escalateConfig tunes the two intermediate waits (small in tests).
type escalateConfig struct {
	sigintWait  time.Duration
	sigtermWait time.Duration
}

// defaultEscalateConfig returns the production escalation bounds.
func defaultEscalateConfig() escalateConfig {
	return escalateConfig{sigintWait: escalateSIGINTWait, sigtermWait: escalateSIGTERMWait}
}

// spawnConfig describes a child to launch in its own process group. When
// lockFile is non-nil its fd is passed to the child so the host folder
// flock is inherited (parent-death safety — plan §3.2/§3.7).
type spawnConfig struct {
	binPath  string
	args     []string
	env      []string
	dir      string
	lockFile *os.File
}

// managedChild is a spawned child in its own process group with its stdio
// pipes and a one-shot Wait. It is the testable unit behind the executor's
// process supervision: the signal-escalation, flock-inheritance, and
// nested-descendant-cleanup Go tests drive it directly with an engineered
// helper child (no Claude).
type managedChild struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	pgid   int

	// releaseWrite is the write end of the gated supervisor's release pipe
	// (finding 4); nil for a non-gated child. Release() writes the go-byte;
	// abortGate() closes it WITHOUT writing so the supervisor EOF-exits without
	// ever running the real command.
	releaseWrite *os.File
	releaseOnce  sync.Once

	done     chan struct{}
	waitErr  error
	waitOnce sync.Once
}

// spawnProcessGroup starts cfg's binary in a fresh process group (setpgid)
// with its stdio wired to pipes and cfg.lockFile inherited when present.
func spawnProcessGroup(cfg spawnConfig) (*managedChild, error) {
	// context.Background: the child's lifecycle is governed by the explicit
	// SIGINT→TERM→KILL Escalate path, not by ctx cancellation, so no
	// CommandContext auto-kill is wanted here.
	//
	//nolint:gosec // remote-owned binary + already-validated args
	cmd := exec.CommandContext(context.Background(), cfg.binPath, cfg.args...)
	cmd.Dir = cfg.dir
	cmd.Env = cfg.env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if cfg.lockFile != nil {
		cmd.ExtraFiles = []*os.File{cfg.lockFile}
	}

	stdin, stdout, stderr, err := commandPipes(cmd)
	if err != nil {
		return nil, err
	}

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("start child: %w", err)
	}

	return &managedChild{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		pgid:   cmd.Process.Pid,
		done:   make(chan struct{}),
	}, nil
}

// spawnGatedGroup launches the RESIDENT GATED supervisor (finding 4) as the
// process-group leader, blocked on a release pipe. The returned child wraps the
// SUPERVISOR (pgid == supervisor pid); the caller records the group identity and
// then calls child.Release() to let the supervisor exec the real command. If
// the caller instead calls child.abortGate() (a bookkeeping failure), the
// supervisor EOF-exits without ever running the command.
func spawnGatedGroup(cfg spawnConfig) (*managedChild, error) {
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("release pipe: %w", err)
	}

	supArgs := append([]string{SupervisorSubcommand}, cfg.args...)

	//nolint:gosec // remote-owned binary + already-validated args
	cmd := exec.CommandContext(context.Background(), cfg.binPath, supArgs...)
	cmd.Dir = cfg.dir
	cmd.Env = cfg.env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.ExtraFiles = []*os.File{readPipe} // → fd 3 in the supervisor

	stdin, stdout, stderr, err := commandPipes(cmd)
	if err != nil {
		_ = readPipe.Close()
		_ = writePipe.Close()

		return nil, err
	}

	err = cmd.Start()
	if err != nil {
		_ = readPipe.Close()
		_ = writePipe.Close()

		return nil, fmt.Errorf("start supervisor: %w", err)
	}

	// The supervisor now holds its own dup of the read end; drop the parent's.
	_ = readPipe.Close()

	return &managedChild{
		cmd:          cmd,
		stdin:        stdin,
		stdout:       stdout,
		stderr:       stderr,
		pgid:         cmd.Process.Pid,
		releaseWrite: writePipe,
		done:         make(chan struct{}),
	}, nil
}

// Release writes the go-byte so the gated supervisor execs the real command.
// A no-op for a non-gated child.
func (c *managedChild) Release() error {
	var writeErr error

	c.releaseOnce.Do(func() {
		if c.releaseWrite == nil {
			return
		}

		_, writeErr = c.releaseWrite.Write([]byte{'G'})
		_ = c.releaseWrite.Close()
		c.releaseWrite = nil
	})

	if writeErr != nil {
		return fmt.Errorf("release supervisor: %w", writeErr)
	}

	return nil
}

// Wait reaps the child exactly once, closing done so a concurrent Escalate
// can observe exit without racing on Wait.
func (c *managedChild) Wait() error {
	c.waitOnce.Do(func() {
		c.waitErr = c.cmd.Wait()
		close(c.done)
	})

	return c.waitErr
}

// Escalate runs the forced SIGINT → SIGTERM → SIGKILL shutdown (plan
// §3.2): SIGINT the group and close stdin (a blocked child EOF-exits),
// then, only if the child survives each bounded wait, escalate to SIGTERM
// and finally SIGKILL. It returns after the child is reaped.
func (c *managedChild) Escalate(cfg escalateConfig) {
	c.signalGroup(syscall.SIGINT)
	c.closeStdin()

	if c.waitUntil(cfg.sigintWait) {
		return
	}

	c.signalGroup(syscall.SIGTERM)

	if c.waitUntil(cfg.sigtermWait) {
		return
	}

	c.signalGroup(syscall.SIGKILL)
	<-c.done
}

// abortGate closes the release pipe WITHOUT the go-byte, so the gated
// supervisor EOF-exits without ever running the real command (fail closed).
func (c *managedChild) abortGate() {
	c.releaseOnce.Do(func() {
		if c.releaseWrite == nil {
			return
		}

		_ = c.releaseWrite.Close()
		c.releaseWrite = nil
	})
}

// signalGroup sends sig to the child's whole process group (negative pid).
func (c *managedChild) signalGroup(sig syscall.Signal) {
	_ = syscall.Kill(-c.pgid, sig)
}

// closeStdin closes the child's stdin so a child blocked on a prompt sees
// EOF and exits (plan §3.2).
func (c *managedChild) closeStdin() {
	_ = c.stdin.Close()
}

// waitUntil reports whether the child exited within d.
func (c *managedChild) waitUntil(duration time.Duration) bool {
	select {
	case <-c.done:
		return true
	case <-time.After(duration):
		return false
	}
}
