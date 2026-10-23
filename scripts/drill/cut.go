package drill

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// Cut exports only the specified layers into an unrelated single-commit repository.
func (r Runner) Cut(ctx context.Context, directory, name string) (Cut, error) {
	if name != "a" && name != "b" && name != "c" && name != "d" {
		return Cut{}, fmt.Errorf("%w: cut must be a, b, c or d", errRefused)
	}

	record, err := Read(directory)
	if err != nil {
		return Cut{}, err
	}

	if _, exists := record.Cuts[name]; exists {
		return Cut{}, fmt.Errorf("%w: cut already exists", errRefused)
	}

	cut, err := createCut(ctx, record, name)
	if err != nil {
		return cut, err
	}

	err = change(directory, func(record *Record) error {
		if _, exists := record.Cuts[name]; exists {
			return fmt.Errorf("%w: concurrent cut creation", errRefused)
		}

		record.Cuts[name] = cut

		return nil
	})
	slog.Info("Drill cut created", "cut", name, "directory", cut.Directory, "error", err)

	return cut, err
}

func validateTree(directory, cut string) error {
	err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walking cut: %w", walkErr)
		}

		if entry.IsDir() {
			return nil
		}

		relative, err := filepath.Rel(directory, path)
		if err != nil {
			return fmt.Errorf("locating cut file: %w", err)
		}

		data, err := disk.Read(path)
		if err != nil {
			return err
		}

		if !kept(relative) || removed(cut, relative, data) || entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%w: unexpected cut file %s", errRefused, relative)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("validating cut: %w", err)
	}

	return nil
}

func createCut(ctx context.Context, record Record, name string) (Cut, error) {
	tree, err := privateDirectory(record.Repository)
	if err != nil {
		return Cut{}, err
	}

	cut := Cut{Directory: tree, Base: record.Base, Digest: record.Digest}

	archive, err := (git.Repository{Dir: record.Repository}).Archive(ctx, record.Base)
	if err != nil {
		return cut, err
	}

	err = exportTree(tree, archive, name)
	if err != nil {
		return cut, err
	}

	err = validateTree(tree, name)
	if err != nil {
		return cut, err
	}

	cut.Commit, err = (git.Repository{Dir: tree}).InitSnapshot(ctx)
	if err != nil {
		return cut, err
	}

	return cut, nil
}
