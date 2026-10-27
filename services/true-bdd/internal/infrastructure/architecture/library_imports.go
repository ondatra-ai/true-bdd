package architecture

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

// ErrTestOnlyLibrary prevents production code from depending on the tests' libraries.
var ErrTestOnlyLibrary = errors.New("imports test-only library")

// CheckLibraryImports reads production imports, including files excluded by the current platform's build tags.
func (a *Architecture) CheckLibraryImports(host string) error {
	for _, service := range a.Services {
		if service.Language != "go" {
			continue
		}

		err := a.checkServiceImports(host, service)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *Architecture) checkServiceImports(host string, service Service) error {
	err := a.walkServiceImports(host, service, filepath.Join(host, service.Path), make(map[string]bool))
	if err != nil {
		return fmt.Errorf("check service libraries: %w", err)
	}

	return nil
}

func (a *Architecture) walkServiceImports(host string, service Service, root string, visited map[string]bool) error {
	root, err := filepath.EvalSymlinks(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("resolve service source: %w", err)
	}

	if visited[root] {
		return nil
	}

	visited[root] = true

	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("read service imports: %w", walkErr)
		}

		linked, err := linkedSourceDirectory(path, entry)
		if err != nil {
			return err
		}

		if linked {
			return a.walkServiceImports(host, service, path, visited)
		}

		if !productionGoSource(path, entry) {
			return nil
		}

		return a.checkFileImports(host, service, path)
	})
	if err != nil {
		return fmt.Errorf("inspect service source: %w", err)
	}

	return nil
}

func (a *Architecture) checkFileImports(host string, service Service, path string) error {
	data, err := disk.Read(path)
	if err != nil {
		return fmt.Errorf("read service source: %w", err)
	}

	// Syntax errors belong to the runner, so a writer can still repair malformed source.
	parsed, _ := parser.ParseFile(token.NewFileSet(), path, data, parser.ImportsOnly)
	if parsed == nil {
		return nil
	}

	for _, imported := range parsed.Imports {
		name, unquoteErr := strconv.Unquote(imported.Path.Value)
		if unquoteErr != nil {
			continue
		}

		if module := a.testOnlyImport(service, name); module != "" {
			relative, _ := filepath.Rel(host, path)

			return fmt.Errorf("service %s: %s %w %s", service.Name, relative, ErrTestOnlyLibrary, module)
		}
	}

	return nil
}

func (a *Architecture) testOnlyImport(service Service, imported string) string {
	module := ""
	for _, library := range a.Testing.Libraries {
		if libraryContains(library.Module, imported) && len(library.Module) > len(module) {
			module = library.Module
		}
	}

	if module == "" {
		return ""
	}

	for _, production := range service.Libraries {
		if libraryContains(production.Module, imported) && len(production.Module) >= len(module) {
			return ""
		}
	}

	return module
}

func linkedSourceDirectory(path string, entry fs.DirEntry) (bool, error) {
	if entry.Type()&os.ModeSymlink == 0 {
		return false, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("read linked service source: %w", err)
	}

	return info.IsDir(), nil
}

func productionGoSource(path string, entry fs.DirEntry) bool {
	return !entry.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
}
