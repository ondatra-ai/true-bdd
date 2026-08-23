package remote

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

const folderLockPerm = 0o644

// errFolderLocked signals that another remote already holds the host
// folder's flock — the run is terminated error(folder_locked).
var errFolderLocked = errors.New("folder already locked by another remote")

// FolderLock is the host-side authoritative folder mutex (plan §3.7): a
// non-blocking flock taken before spawning a command child. The underlying
// fd is inherited by the child, so the lock survives until both release it (parent-death safety).
type FolderLock struct {
	file *os.File
}

// AcquireFolderLock takes a non-blocking exclusive flock on path.
// Returns errFolderLocked when another holder owns it.
func AcquireFolderLock(path string) (*FolderLock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, folderLockPerm)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		_ = file.Close()

		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errFolderLocked
		}

		return nil, fmt.Errorf("flock: %w", err)
	}

	return &FolderLock{file: file}, nil
}

// File returns the locked file so its fd can be inherited by the child.
func (l *FolderLock) File() *os.File {
	return l.file
}

// Release unlocks and closes the flock file.
func (l *FolderLock) Release() {
	_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	_ = l.file.Close()
}
