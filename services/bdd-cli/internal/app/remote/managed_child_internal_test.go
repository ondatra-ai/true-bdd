package remote

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// spawnHelper re-execs THIS test binary as an engineered helper child in
// the given mode, optionally inheriting a flock fd, and waits until the
// child has installed its signal handlers.
func spawnHelper(t *testing.T, mode string, lockFile *os.File, extraEnv ...string) *managedChild {
	t.Helper()

	env := append(os.Environ(), testChildModeEnv+"="+mode)
	env = append(env, extraEnv...)

	child, err := spawnProcessGroup(spawnConfig{
		binPath:  os.Args[0],
		env:      env,
		dir:      t.TempDir(),
		lockFile: lockFile,
	})
	if err != nil {
		t.Fatalf("spawn helper %q: %v", mode, err)
	}

	waitReady(t, child)

	return child
}

// waitReady blocks until the helper child writes its READY marker so a
// subsequent signal never races the child's signal.Ignore setup.
func waitReady(t *testing.T, child *managedChild) {
	t.Helper()

	ready := make(chan struct{})

	go func() {
		reader := bufio.NewReader(child.stdout)
		for {
			line, err := reader.ReadString('\n')
			if strings.Contains(line, testChildReady) {
				close(ready)

				return
			}

			if err != nil {
				return
			}
		}
	}()

	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("helper child never signalled READY")
	}
}

// reap starts a goroutine that reaps the child, returning a channel of its
// wait error so Escalate (which waits on the child's done channel) never
// deadlocks.
func reap(child *managedChild) <-chan error {
	waitCh := make(chan error, 1)
	go func() { waitCh <- child.Wait() }()

	return waitCh
}

// TestFolderLockInheritedByChild proves the parent-death safety of the
// host folder flock (plan §3.7): the inherited fd keeps the lock held even
// after the parent's own handle is gone, until the child exits.
func TestFolderLockInheritedByChild(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "true-bdd-harness.lock")

	lock, err := AcquireFolderLock(lockPath)
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}

	child := spawnHelper(t, modeBlockStdin, lock.File())

	// Simulate the remote dying WITHOUT unlocking: close the parent's fd
	// (no LOCK_UN). The child's inherited fd keeps the description — and
	// thus the flock — alive.
	_ = lock.File().Close()

	_, err = AcquireFolderLock(lockPath)
	if !errors.Is(err, errFolderLocked) {
		t.Fatalf("expected lock held by child, got %v", err)
	}

	// The child EOF-exits when its stdin closes; the lock then frees.
	child.closeStdin()

	waitErr := child.Wait()
	if waitErr != nil {
		t.Fatalf("child exit: %v", waitErr)
	}

	freed, err := AcquireFolderLock(lockPath)
	if err != nil {
		t.Fatalf("lock should be free after child exit, got %v", err)
	}

	freed.Release()
}

// TestEscalateSigintEOFExitsCleanly proves the soft shutdown path (plan
// §3.2): SIGINT + stdin close lets a child that honors EOF exit cleanly,
// with no forced SIGTERM/SIGKILL.
func TestEscalateSigintEOFExitsCleanly(t *testing.T) {
	child := spawnHelper(t, modeBlockStdin, nil)
	waitCh := reap(child)
	start := time.Now()

	child.Escalate(escalateConfig{sigintWait: 3 * time.Second, sigtermWait: time.Second})

	elapsed := time.Since(start)

	waitErr := <-waitCh
	if waitErr != nil {
		t.Fatalf("child should exit cleanly on stdin EOF, got %v", waitErr)
	}

	if elapsed >= 3*time.Second {
		t.Fatalf("clean EOF exit took %v; escalation should not have waited the full SIGINT budget", elapsed)
	}
}

// TestEscalateForcedTermKill proves the FORCED escalation (plan §3.2 / §4.6)
// on a child that ignores SIGINT and SIGTERM: only SIGKILL stops it, and
// the escalation completes within the bounded budget.
func TestEscalateForcedTermKill(t *testing.T) {
	child := spawnHelper(t, modeIgnoreSignals, nil)
	waitCh := reap(child)
	start := time.Now()

	child.Escalate(escalateConfig{sigintWait: 250 * time.Millisecond, sigtermWait: 250 * time.Millisecond})

	elapsed := time.Since(start)

	waitErr := <-waitCh
	if !killedBySignal(waitErr, syscall.SIGKILL) {
		t.Fatalf("child should have been SIGKILLed, got %v", waitErr)
	}

	if elapsed > 5*time.Second {
		t.Fatalf("forced escalation took %v; expected a bounded completion", elapsed)
	}
}

// TestGroupSignalReapsNestedDescendant proves nested-descendant cleanup
// (plan §3.2): signalling the child's process group reaps a grandchild that
// inherited the group, which is exactly what the pids-file teardown does.
func TestGroupSignalReapsNestedDescendant(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	child := spawnHelper(t, modeSpawnGrandchild, nil, testGrandchildPidEnv+"="+pidFile)

	grandchildPid := waitForPid(t, pidFile)
	if !processAlive(grandchildPid) {
		t.Fatalf("grandchild %d not alive", grandchildPid)
	}

	// Signal the whole group (what the harness teardown does with the pids
	// file's PGID) — the grandchild in the same group dies with it.
	child.signalGroup(syscall.SIGKILL)
	_ = child.Wait()

	deadline := time.Now().Add(3 * time.Second)
	for processAlive(grandchildPid) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}

	if processAlive(grandchildPid) {
		t.Fatalf("grandchild %d survived the group kill", grandchildPid)
	}
}

// killedBySignal reports whether a wait error is a signal-kill by sig.
func killedBySignal(waitErr error, sig syscall.Signal) bool {
	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) {
		return false
	}

	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		return false
	}

	return status.Signaled() && status.Signal() == sig
}

// processAlive reports whether pid is still running.
func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// waitForPid polls until the helper child writes its grandchild pid.
func waitForPid(t *testing.T, pidFile string) int {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(pidFile)
		if err == nil && len(strings.TrimSpace(string(data))) > 0 {
			pid, convErr := strconv.Atoi(strings.TrimSpace(string(data)))
			if convErr == nil {
				return pid
			}
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("grandchild pid file never appeared")

	return 0
}
