package disk

import (
	"fmt"
	"os"
)

// Executable grants the owner execute permission on a prepared binary.
func Executable(path string) error {
	held, err := acquire(path, true)
	if err != nil {
		return err
	}
	defer held.release()

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat executable %s: %w", path, err)
	}

	const ownerExecute = 0o100

	err = os.Chmod(path, info.Mode().Perm()|ownerExecute)
	if err != nil {
		return fmt.Errorf("making %s executable: %w", path, err)
	}

	return nil
}
