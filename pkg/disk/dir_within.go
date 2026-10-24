package disk

import (
	"errors"
	"fmt"
	"os"
)

// DirWithin creates a relative directory and its missing parents beneath root,
// refusing symlinks that escape it, including during creation. Like Dir it takes
// no hold. The result reports whether the directory was absent before creation.
func DirWithin(root, path string, mode Mode) (bool, error) {
	base, err := os.OpenRoot(root)
	if err != nil {
		return false, fmt.Errorf("opening directory root %s: %w", root, err)
	}
	defer func() { _ = base.Close() }()

	_, err = base.Stat(path)

	missing := errors.Is(err, os.ErrNotExist)
	if err != nil && !missing {
		return false, fmt.Errorf("checking directory %s beneath %s: %w", path, root, err)
	}

	err = base.MkdirAll(path, mode.dirPerm())
	if err != nil {
		return false, fmt.Errorf("creating directory %s beneath %s: %w", path, root, err)
	}

	return missing, nil
}
