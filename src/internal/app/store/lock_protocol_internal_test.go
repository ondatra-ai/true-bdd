package store

// Linearizable scan-vs-command lock protocol (plan §1.5, round-3 fix). Exact
// acquisition order: a SCAN takes scan.lock shared then probes
// command-intent.lock (held ⇒ inventory_busy); a COMMAND takes
// command-intent.lock EXCLUSIVE (fail-fast folder_locked vs another command),
// then drains in-flight scans by taking scan.lock exclusive, then the folder
// flock. Linearization point: either a scan sees the planted intent, or the
// command waits for that scan — no TOCTOU window.

import (
	"testing"
	"time"
)

func TestLockCommandVsCommandFailFast(t *testing.T) {
	locks := requireLocks(t, t.TempDir())

	first, o1 := locks.BeginCommand()
	if o1 != commandOK {
		t.Fatalf("first command: %s, want ok", o1)
	}
	defer first.Release()

	// A second command finds the exclusive intent lock held ⇒ fail-fast.
	second, o2 := locks.BeginCommand()
	if o2 != commandFolderLock {
		t.Fatalf("second command: %s, want folder_locked", o2)
	}

	if second != nil {
		second.Release()
	}
}

func TestLockScanBusyWhileCommandHeld(t *testing.T) {
	locks := requireLocks(t, t.TempDir())

	cmd, o := locks.BeginCommand()
	if o != commandOK {
		t.Fatalf("command: %s", o)
	}

	// A scan probing the planted command-intent returns inventory_busy — it
	// never presents a torn view while the command mutates the folder.
	if _, so := locks.BeginScan(); so != scanInventoryBusy {
		t.Fatalf("scan while command held: %s, want inventory_busy", so)
	}

	cmd.Release()

	// Once the command releases, a fresh scan proceeds.
	scan, so := locks.BeginScan()
	if so != scanOK {
		t.Fatalf("scan after command release: %s, want ok", so)
	}

	scan.Release()
}

func TestLockCommandWaitsForInFlightScan(t *testing.T) {
	locks := requireLocks(t, t.TempDir())

	// A shared scan is in flight (holding scan.lock shared).
	scan, so := locks.BeginScan()
	if so != scanOK {
		t.Fatalf("scan: %s", so)
	}

	type cmdResult struct {
		lease   Lease
		outcome CommandOutcome
	}

	done := make(chan cmdResult, 1)

	go func() {
		lease, o := locks.BeginCommand()
		done <- cmdResult{lease, o}
	}()

	// The command plants intent then blocks acquiring scan.lock exclusive: it
	// must WAIT for the in-flight scan to drain (no window to start under it).
	select {
	case <-done:
		t.Fatal("command must wait while a shared scan is in flight")
	case <-time.After(200 * time.Millisecond):
	}

	// Releasing the scan lets the command proceed and succeed.
	scan.Release()

	select {
	case res := <-done:
		if res.outcome != commandOK {
			t.Fatalf("command after scan drained: %s, want ok", res.outcome)
		}

		if res.lease != nil {
			res.lease.Release()
		}
	case <-time.After(2 * time.Second):
		t.Fatal("command must proceed after the scan releases")
	}
}
