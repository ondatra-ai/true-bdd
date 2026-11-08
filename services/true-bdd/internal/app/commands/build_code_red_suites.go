package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
)

// hasCodeRedSuite identifies Steps packages even when the Registry cannot be read.
func hasCodeRedSuite(arch *architecture.Architecture) (bool, error) {
	host, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("locate Go suites: %w", err)
	}

	roots, err := arch.TestDirectories(host, nil)
	if err != nil {
		return false, err
	}

	args, err := testrunner.SplitCommand(arch.Testing.Commands.Replay)
	if err != nil {
		return false, err
	}

	if slices.Contains(args, "./...") {
		roots = append(roots, ".")
	}

	excluded := make(map[string]bool)
	for _, service := range arch.Services {
		excluded[filepath.Join(host, service.Path)] = true
	}

	for _, root := range roots {
		found, err := containsStepsPackage(filepath.Join(host, root), excluded)
		if err != nil || found {
			return found, err
		}
	}

	return false, nil
}

func containsStepsPackage(root string, excluded map[string]bool) (bool, error) {
	found := false

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !entry.IsDir() {
			return nil
		}

		name := entry.Name()
		if excluded[path] || (path != root && skipRedSuiteDirectory(name)) {
			return filepath.SkipDir
		}

		if name == "steps" {
			found = true

			return filepath.SkipAll
		}

		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("inspect Go suites: %w", err)
	}

	return found, nil
}

func skipRedSuiteDirectory(name string) bool {
	return strings.HasPrefix(name, ".") || name == "fixtures" || name == "tmp" ||
		name == "node_modules" || name == "vendor"
}
