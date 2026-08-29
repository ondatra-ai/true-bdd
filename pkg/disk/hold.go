package disk

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const (
	// holdWait is far beyond any legitimate short access, so exceeding it is a
	// diagnosis rather than contention — see waitFor.
	holdWait = 10 * time.Second
	holdPoll = 5 * time.Millisecond
)

// errHeld is returned when the wait expires.
var errHeld = errors.New("directory lock not released")

// hold is the advisory flock on a target's parent directory, plus the handle
// it was taken on — which doubles as the fsync target a rename needs.
type hold struct {
	dir *os.File
}

// acquire locks target's parent directory, shared for a read and exclusive for
// anything that mutates.
func acquire(target string, exclusive bool) (*hold, error) {
	parent := filepath.Dir(target)

	handle, err := os.Open(parent)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", parent, err)
	}

	how := syscall.LOCK_SH
	if exclusive {
		how = syscall.LOCK_EX
	}

	err = waitFor(handle, how, holdWait)
	if err != nil {
		_ = handle.Close()

		return nil, err
	}

	return &hold{dir: handle}, nil
}

// waitFor polls rather than blocking outright, so re-entering this package
// from inside an Update callback reports itself instead of hanging forever —
// flock binds to the descriptor, so a second hold blocks its own goroutine.
func waitFor(handle *os.File, how int, wait time.Duration) error {
	deadline := time.Now().Add(wait)

	for {
		err := syscall.Flock(int(handle.Fd()), how|syscall.LOCK_NB)
		if err == nil {
			return nil
		}

		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return fmt.Errorf("locking %s: %w", handle.Name(), err)
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("%w: %s held for %s — did an Update callback log, "+
				"or otherwise call back into this package?", errHeld, handle.Name(), wait)
		}

		time.Sleep(holdPoll)
	}
}

// release drops the lock. Closing the descriptor is what releases the flock.
func (h *hold) release() {
	_ = h.dir.Close()
}
