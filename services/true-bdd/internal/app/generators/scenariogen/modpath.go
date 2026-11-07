package scenariogen

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

// suiteImportPath computes the import path of the suite itself
// by walking up to the nearest go.mod — not assumed, since a sentinel module
// (every fixture tree) gives a suite inside it an unrelated module path.
func suiteImportPath(repoRoot, suitePath string) (string, error) {
	suiteDir := filepath.Join(repoRoot, filepath.FromSlash(suitePath))

	moduleDir, modulePath, err := findModule(suiteDir)
	if err != nil {
		return "", err
	}

	relative, err := filepath.Rel(moduleDir, suiteDir)
	if err != nil {
		return "", fmt.Errorf("locate %s inside %s: %w", suiteDir, moduleDir, err)
	}

	return path.Join(modulePath, filepath.ToSlash(relative)), nil
}

// findModule returns the nearest directory at or above dir holding a
// go.mod, and that file's module path.
func findModule(dir string) (string, string, error) {
	for current := dir; ; current = filepath.Dir(current) {
		modulePath, found, err := readModulePath(filepath.Join(current, "go.mod"))
		if err != nil {
			return "", "", err
		}

		if found {
			return current, modulePath, nil
		}

		if current == filepath.Dir(current) {
			return "", "", fmt.Errorf("%s: %w", dir, ErrNoModuleForSuite)
		}
	}
}

// readModulePath reads the `module` directive of a go.mod, reporting
// whether the file was there at all.
func readModulePath(modFile string) (string, bool, error) {
	data, err := disk.Read(modFile)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}

	if err != nil {
		return "", false, fmt.Errorf("read %s: %w", modFile, err)
	}

	for line := range strings.SplitSeq(string(data), "\n") {
		trimmed := strings.TrimSpace(line)

		after, isModule := strings.CutPrefix(trimmed, "module ")
		if !isModule {
			continue
		}

		return strings.TrimSpace(after), true, nil
	}

	return "", false, fmt.Errorf("%s: %w", modFile, ErrNoModuleDirective)
}
