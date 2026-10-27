package errors

import (
	"errors"
	"fmt"
	"io/fs"
)

// ProjectFile distinguishes missing init resources from files a host must repair.
func ProjectFile(path string, cause error) error {
	if errors.Is(cause, fs.ErrNotExist) {
		return fmt.Errorf("%s is missing; run true-bdd init: %w", path, cause)
	}

	return fmt.Errorf("%w; fix file %s", cause, path)
}
