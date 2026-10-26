package architecture

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
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
	err := filepath.WalkDir(filepath.Join(host, service.Path), func(path string, entry fs.DirEntry, walkErr error) error {
		if errors.Is(walkErr, fs.ErrNotExist) {
			return nil
		}

		if walkErr != nil {
			return fmt.Errorf("read service imports: %w", walkErr)
		}

		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		return a.checkFileImports(host, service, path)
	})
	if err != nil {
		return fmt.Errorf("check service libraries: %w", err)
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
