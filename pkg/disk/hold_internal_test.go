package disk

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// testWait is short enough that a run respecting it is distinguishable from
// one that fell back to holdWait.
const testWait = 50 * time.Millisecond

// The wait is a parameter so this path is testable at all: an Update callback
// that calls back into this package deadlocks against its own hold, and must
// say so rather than stall.
func TestWaitForGivesUpOnAHeldDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	held, err := acquire(filepath.Join(dir, "target"), true)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	defer held.release()

	second, err := os.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	defer func() { _ = second.Close() }()

	start := time.Now()

	err = waitFor(second, syscall.LOCK_EX, testWait)
	if !errors.Is(err, errHeld) {
		t.Fatalf("waitFor on a held directory = %v, want errHeld", err)
	}

	if waited := time.Since(start); waited > time.Second {
		t.Fatalf("waited %s — the parameter was ignored for holdWait", waited)
	}
}
