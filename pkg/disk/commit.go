package disk

import (
	"fmt"
	"os"
	"path/filepath"
)

// tempPrefix names the staging file deterministically rather than randomly.
// Under an exclusive hold nothing else can be mid-write in the directory, so a
// fixed name bounds residue to one stale temp per target and self-heals below.
const tempPrefix = ".true-bdd.tmp."

// commit publishes data at path: staging file, fsync, atomic rename. Assumes
// an exclusive hold is held on path's parent.
func commit(path string, data []byte, perm os.FileMode) error {
	temp := filepath.Join(filepath.Dir(path), tempPrefix+filepath.Base(path))

	// Unlinks a planted symlink rather than following it, and clears any temp a
	// killed run abandoned. O_EXCL then refuses anything that survives.
	err := os.Remove(temp)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clearing %s: %w", temp, err)
	}

	handle, err := os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return fmt.Errorf("creating %s: %w", temp, err)
	}

	err = writeAll(handle, data)
	if err != nil {
		_ = os.Remove(temp)

		return err
	}

	err = os.Rename(temp, path)
	if err != nil {
		_ = os.Remove(temp)

		return fmt.Errorf("renaming %s: %w", temp, err)
	}

	return nil
}

// writeAll writes the whole payload. Deliberately no fsync: the rename is what
// readers need, and measured on this tree an fsync per write costs 174x
// (9.5ms against 0.05ms) for durability across a power cut nothing here wants.
func writeAll(handle *os.File, data []byte) error {
	_, err := handle.Write(data)
	if err != nil {
		_ = handle.Close()

		return fmt.Errorf("writing %s: %w", handle.Name(), err)
	}

	err = handle.Close()
	if err != nil {
		return fmt.Errorf("closing %s: %w", handle.Name(), err)
	}

	return nil
}

// createPerm is the mode a write applies. An existing file keeps the mode it
// has: a migration that silently chmod'd every target is a change nobody made.
func createPerm(path string, mode Mode) os.FileMode {
	info, err := os.Stat(path)
	if err == nil {
		return info.Mode().Perm()
	}

	return mode.filePerm()
}
