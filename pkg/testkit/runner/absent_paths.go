package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// ErrAbsentPathUnsafe is returned for an absent path that is empty,
// absolute or leads out of the run dir: it is never deleted.
var ErrAbsentPathUnsafe = errors.New("an absent path must be relative and inside the run dir")

// ErrAbsentPathNotHeld is returned when the assembled run dir does not
// hold a path the scenario says is absent: a typo would otherwise pass.
var ErrAbsentPathNotHeld = errors.New("the assembled run dir holds no such path to leave out")

// dropAbsentPaths deletes each path the scenario says the tree has no
// copy of, once the repo layer and the input overlay are in place.
func dropAbsentPaths(fixture *Fixture, tmpDir string) error {
	for _, rel := range fixture.Absent {
		if !filepath.IsLocal(rel) {
			return fmt.Errorf("%w: %q", ErrAbsentPathUnsafe, rel)
		}

		path := filepath.Join(tmpDir, rel)

		_, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrAbsentPathNotHeld, rel)
		}

		err = disk.RemoveTree(path)
		if err != nil {
			return fmt.Errorf("leave out %s: %w", rel, err)
		}
	}

	return nil
}
