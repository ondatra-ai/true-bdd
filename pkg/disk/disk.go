package disk

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Read returns the whole file under a shared hold.
func Read(path string) ([]byte, error) {
	held, err := acquire(path, false)
	if err != nil {
		return nil, err
	}
	defer held.release()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	return data, nil
}

// ReadFrom returns the bytes from offset to end of file under a shared hold —
// a tailer's read, which a concurrent Append can no longer tear. A missing
// file wraps os.ErrNotExist: nothing appended yet is a normal first poll.
func ReadFrom(path string, offset int64) ([]byte, error) {
	data, err := Read(path)
	if err != nil {
		return nil, err
	}

	if offset >= int64(len(data)) {
		return nil, nil
	}

	return data[offset:], nil
}

// Write replaces the whole file, creating its directory. mode applies on
// create only; an existing file keeps the mode it has.
func Write(path string, data []byte, mode Mode) error {
	err := Dir(filepath.Dir(path), mode)
	if err != nil {
		return err
	}

	held, err := acquire(path, true)
	if err != nil {
		return err
	}
	defer held.release()

	return commit(path, data, createPerm(path, mode))
}

// Append adds one record in ONE write, newline included: a second write for
// the newline could interleave with a concurrent Append and tear both lines.
// Not Write — a rename would swap the inode from under a tailer.
func Append(path string, record []byte, mode Mode) error {
	err := Dir(filepath.Dir(path), mode)
	if err != nil {
		return err
	}

	held, err := acquire(path, true)
	if err != nil {
		return err
	}
	defer held.release()

	handle, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, mode.filePerm())
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}

	defer func() { _ = handle.Close() }()

	_, err = handle.Write(append(record, '\n'))
	if err != nil {
		return fmt.Errorf("appending to %s: %w", path, err)
	}

	return nil
}

// Update reads, changes and writes under one exclusive hold. change must not
// call back in, and slog counts once pkg/logging is installed: its JSON sink
// appends through this package, so a logging callback deadlocks itself.
func Update(path string, mode Mode, change func(before []byte) ([]byte, error)) error {
	err := Dir(filepath.Dir(path), mode)
	if err != nil {
		return err
	}

	held, err := acquire(path, true)
	if err != nil {
		return err
	}
	defer held.release()

	before, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	after, err := change(before)
	if err != nil {
		return fmt.Errorf("updating %s: %w", path, err)
	}

	return commit(path, after, createPerm(path, mode))
}

// Dir creates path and every missing parent. Taken without a hold: locking a
// directory that may not exist has nothing to lock, and MkdirAll is idempotent
// under concurrency.
func Dir(path string, mode Mode) error {
	err := os.MkdirAll(path, mode.dirPerm())
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}

	return nil
}

// Ensure creates path empty when it is absent and leaves an existing file
// untouched. A log sink calls it so the file exists from the moment it is
// installed, whether or not a record is ever written.
func Ensure(path string, mode Mode) error {
	err := Dir(filepath.Dir(path), mode)
	if err != nil {
		return err
	}

	held, err := acquire(path, true)
	if err != nil {
		return err
	}
	defer held.release()

	handle, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, mode.filePerm())
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}

	err = handle.Close()
	if err != nil {
		return fmt.Errorf("closing %s: %w", path, err)
	}

	return nil
}

// TempDir creates a uniquely named directory under parent for scratch work.
// Taken without a hold: the name does not exist until this returns it, so
// nothing else can be contending for it.
func TempDir(parent, pattern string) (string, error) {
	path, err := os.MkdirTemp(parent, pattern)
	if err != nil {
		return "", fmt.Errorf("creating a temp dir under %s: %w", parent, err)
	}

	return path, nil
}

// Remove deletes path, reporting nothing when it was already absent — which
// includes the case where its directory never existed, so callers keep
// os.Remove's semantics.
func Remove(path string) error {
	held, err := acquire(path, true)
	if err != nil {
		return skipMissing(err)
	}
	defer held.release()

	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", path, err)
	}

	return nil
}

// RemoveTree deletes path and everything under it. Held on path's parent, not
// on path, which is about to stop existing.
func RemoveTree(path string) error {
	held, err := acquire(path, true)
	if err != nil {
		return skipMissing(err)
	}
	defer held.release()

	err = os.RemoveAll(path)
	if err != nil {
		return fmt.Errorf("removing %s: %w", path, err)
	}

	return nil
}

// Copy writes src's contents to dst.
func Copy(dst, src string, mode Mode) error {
	data, err := Read(src)
	if err != nil {
		return err
	}

	return Write(dst, data, mode)
}

// skipMissing turns "the directory is not there" into success for the two
// verbs that were asked to make something not be there.
func skipMissing(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}
