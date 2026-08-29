package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

const (
	scanLockName    = "scan.lock"
	intentLockName  = "command-intent.lock"
	folderLockName  = "folder.lock"
	commandDrainTTL = 30 * time.Second
	drainPoll       = 10 * time.Millisecond
)

// Locks models the two host-tmp lock files (scan.lock, command-intent.lock)
// plus the folder flock. flock is per-open-file-description, so each
// acquisition opens its own fd; a lease's Release closes them to release the OS locks.
type Locks struct {
	dir        string
	scanPath   string
	intentPath string
	folderPath string
}

// OpenLocks prepares the lock files under dir.
func OpenLocks(dir string) (*Locks, error) {
	err := disk.Dir(dir, disk.Shared)
	if err != nil {
		return nil, fmt.Errorf("locks: mkdir: %w", err)
	}

	locks := &Locks{
		dir:        dir,
		scanPath:   filepath.Join(dir, scanLockName),
		intentPath: filepath.Join(dir, intentLockName),
		folderPath: filepath.Join(dir, folderLockName),
	}

	for _, p := range []string{locks.scanPath, locks.intentPath, locks.folderPath} {
		//nolint:forbidigo // a lease handle held across a scan, not a file access.
		//nolint:forbidigo // a lease handle held across a scan, not a file access.
		f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, filePerm)
		if err != nil {
			return nil, fmt.Errorf("locks: touch %s: %w", p, err)
		}

		_ = f.Close()
	}

	return locks, nil
}

type heldLease struct {
	mu    sync.Mutex
	files []*os.File
}

func (h *heldLease) Release() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, f := range h.files {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}

	h.files = nil
}

// isLease marks *heldLease as the production LeaseHandle, keeping that
// interface structurally distinct from the test-only Lease seam.
func (h *heldLease) isLease() {}

// BeginScan takes scan.lock SHARED then probes command-intent.lock
// nonblocking-shared: a planted command intent ⇒ inventory_busy (plan §1.5).
func (l *Locks) BeginScan() (LeaseHandle, string) {
	scanFile, err := openLock(l.scanPath)
	if err != nil {
		return nil, lockInventoryBusy
	}

	err = syscall.Flock(int(scanFile.Fd()), syscall.LOCK_SH)
	if err != nil {
		_ = scanFile.Close()

		return nil, lockInventoryBusy
	}

	// Probe command intent while HOLDING the shared scan.lock (linearization
	// point): a held exclusive intent fails the shared probe.
	probe, err := openLock(l.intentPath)
	if err != nil {
		release(scanFile)

		return nil, lockInventoryBusy
	}

	err = syscall.Flock(int(probe.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
	if err != nil {
		_ = probe.Close()

		release(scanFile)

		return nil, lockInventoryBusy
	}

	// The probe succeeded; release it (we only needed to observe intent).
	release(probe)

	return &heldLease{files: []*os.File{scanFile}}, lockOK
}

// BeginCommand plants command-intent.lock EXCLUSIVE (fail-fast folder_locked
// vs another command), drains in-flight scans by taking scan.lock EXCLUSIVE,
// then takes the folder flock (plan §1.5).
func (l *Locks) BeginCommand() (LeaseHandle, string) {
	intentFile, err := openLock(l.intentPath)
	if err != nil {
		return nil, lockFolderLocked
	}

	err = syscall.Flock(int(intentFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		_ = intentFile.Close()

		return nil, lockFolderLocked
	}

	// Drain in-flight scans: take scan.lock EXCLUSIVE with a bounded wait, then
	// release immediately — planted intent already blocks NEW scans, so this
	// only waits out scans that started before the intent was planted.
	if !drainScans(l.scanPath) {
		release(intentFile)

		return nil, lockTimeout
	}

	folderFile, err := openLock(l.folderPath)
	if err != nil {
		release(intentFile)

		return nil, lockFolderLocked
	}

	err = syscall.Flock(int(folderFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		_ = folderFile.Close()

		release(intentFile)

		return nil, lockFolderLocked
	}

	return &heldLease{files: []*os.File{intentFile, folderFile}}, lockOK
}

// drainScans acquires scan.lock EXCLUSIVE (polling within the bounded wait) to
// wait for any in-flight shared scan to finish, then releases it.
func drainScans(scanPath string) bool {
	file, err := openLock(scanPath)
	if err != nil {
		return false
	}
	defer func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}()

	deadline := time.Now().Add(commandDrainTTL)

	for {
		flockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if flockErr == nil {
			return true
		}

		if time.Now().After(deadline) {
			return false
		}

		time.Sleep(drainPoll)
	}
}

func openLock(path string) (*os.File, error) {
	//nolint:forbidigo // a lease handle held across a scan, not a file access.
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, filePerm)
	if err != nil {
		return nil, fmt.Errorf("open lock %s: %w", path, err)
	}

	return file, nil
}

func release(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()
}
