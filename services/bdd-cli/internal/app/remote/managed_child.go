package remote

import (
	"fmt"
	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/truebdd"
	"io"
	"os"
	"sync"
	"syscall"
	"time"
)

// escalateSIGINTWait and escalateSIGTERMWait are Escalate's production
// shutdown bounds (plan §3.2); shortened in tests via escalateConfig.
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
	// binary is the true-bdd to spawn. The zero value is this executable,
	// which is every production caller; the gated-supervisor regressions set
	// it to a built one.
	binary   truebdd.Binary
	args     []string
	env      []string
	dir      string
	lockFile *os.File
}

// managedChild is a spawned child in its own process group with its stdio
// pipes and a one-shot Wait — the testable unit behind the executor's
// process supervision (signal-escalation, flock-inheritance, cleanup).
type managedChild struct {
	proc   *cli.Process
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	pgid   int

	// releaseWrite is the write end of the gated supervisor's release pipe
	// (finding 4); nil for a non-gated child. Release() writes the go-byte
	// to exec the command; see abortGate for the fail-closed abort path.
	releaseWrite *os.File
	releaseOnce  sync.Once

	done       chan struct{}
	waitErr    error
	waitResult cli.Result
	waitOnce   sync.Once
}

// spawnProcessGroup starts cfg's binary in a fresh process group (setpgid)
// with its stdio wired to pipes and cfg.lockFile inherited when present.
func spawnProcessGroup(cfg spawnConfig) (*managedChild, error) {
	proc, err := cfg.binary.StartGroup(truebdd.Child{
		Args: cfg.args, Dir: cfg.dir, Env: cfg.env, Lock: cfg.lockFile,
	})
	if err != nil {
		return nil, fmt.Errorf("start child: %w", err)
	}

	return &managedChild{
		proc:   proc,
		stdin:  proc.Stdin,
		stdout: proc.Stdout,
		stderr: proc.Stderr,
		pgid:   proc.Pid(),
		done:   make(chan struct{}),
	}, nil
}

// spawnGatedGroup launches the RESIDENT GATED supervisor (finding 4) as the
// process-group leader (pgid == supervisor pid), blocked on its release pipe
// until the caller calls Release() (exec the command) or abortGate() (fail closed).
func spawnGatedGroup(cfg spawnConfig) (*managedChild, error) {
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("release pipe: %w", err)
	}

	proc, err := cfg.binary.StartGated(truebdd.Child{
		Args: cfg.args, Dir: cfg.dir, Env: cfg.env,
	}, readPipe)
	if err != nil {
		_ = readPipe.Close()
		_ = writePipe.Close()

		return nil, fmt.Errorf("start supervisor: %w", err)
	}

	// The supervisor now holds its own dup of the read end; drop the parent's.
	_ = readPipe.Close()

	return &managedChild{
		proc:         proc,
		stdin:        proc.Stdin,
		stdout:       proc.Stdout,
		stderr:       proc.Stderr,
		pgid:         proc.Pid(),
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
		c.waitResult, c.waitErr = c.proc.Wait()
		// pkg/shell reports a non-zero exit as Result.Code, not an error.
		// This layer's callers key on waitErr != nil, so it is restored here.
		if c.waitErr == nil {
			c.waitErr = c.waitResult.Err()
		}

		close(c.done)
	})

	return c.waitErr
}

// Result is how the child ended, for the callers that need the exit code and
// signal rather than only whether it failed.
func (c *managedChild) Result() cli.Result {
	return c.waitResult
}

// Escalate runs the forced SIGINT→TERM→KILL shutdown (plan §3.2): SIGINT +
// stdin close first, since a healthy claude EOF-exits from that alone; each
// further stage escalates only if the child survives the prior bounded wait.
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
