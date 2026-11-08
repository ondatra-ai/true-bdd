package architecture

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
)

// HasCode reports source files, even incomplete ones, in this service's declared roots.
func (s Service) HasCode(root string) (bool, error) {
	paths := append([]string{s.Path}, s.Writable...)
	for _, path := range paths {
		built, err := sourceUnder(filepath.Join(root, path), s.Language)
		if err != nil {
			return false, err
		}

		if built {
			return true, nil
		}
	}

	return false, nil
}

func sourceUnder(root, language string) (bool, error) {
	found := false

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if errors.Is(walkErr, fs.ErrNotExist) {
			return nil
		}

		if walkErr != nil {
			return walkErr
		}

		if entry.Type().IsRegular() && sourceExtension(language, filepath.Ext(path)) {
			found = true

			return fs.SkipAll
		}

		return nil
	})
	if err != nil {
		return false, fmt.Errorf("inspect service source %s: %w", root, err)
	}

	return found, nil
}

func sourceExtension(language, extension string) bool {
	switch language {
	case "go":
		return extension == ".go"
	case "typescript":
		return slices.Contains([]string{".ts", ".tsx"}, extension)
	case "javascript":
		return slices.Contains([]string{".js", ".jsx", ".mjs", ".cjs"}, extension)
	default:
		return true
	}
}
