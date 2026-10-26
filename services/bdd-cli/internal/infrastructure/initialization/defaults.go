// Package initialization carries files only for init, never for runtime loading.
package initialization

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

//go:embed files

var defaults embed.FS

// File reports whether init created a path or left it untouched.
type File struct {
	Path    string
	Created bool
}

// WriteMissing installs each absent default in the current host project.
func WriteMissing() ([]File, error) {
	var results []File

	err := fs.WalkDir(defaults, "files", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("list init files: %w", walkErr)
		}

		if entry.IsDir() {
			return nil
		}

		target := path[len("files/"):]

		created, err := writeMissing(target, path)
		if err != nil {
			return err
		}

		results = append(results, File{Path: target, Created: created})

		return nil
	})
	if err != nil {
		return results, fmt.Errorf("initialize project: %w", err)
	}

	return results, nil
}

func writeMissing(target, source string) (bool, error) {
	_, err := os.Lstat(target)
	if err == nil {
		return false, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("inspect %s: %w", target, err)
	}

	content, err := defaults.ReadFile(source)
	if err != nil {
		return false, fmt.Errorf("read init copy %s: %w", source, err)
	}

	err = disk.Update(target, disk.Shared, func(_ []byte) ([]byte, error) {
		_, statErr := os.Lstat(target)
		if statErr == nil {
			return nil, fs.ErrExist
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return nil, fmt.Errorf("inspect %s: %w", target, statErr)
		}

		return content, nil
	})
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("create %s: %w", target, err)
	}

	return true, nil
}
