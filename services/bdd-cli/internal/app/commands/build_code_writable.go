package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner"
)

func codeWritableRoots(deps BuildCodeDeps, arch *architecture.Architecture) ([]architecture.WritableRoot, error) {
	host, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("locate host: %w", err)
	}

	tests := []string{}

	for _, scenario := range codeRegistry(deps.DocResolver) {
		if scenario.Path != "" {
			tests = append(tests, filepath.Dir(scenario.Path))
		}
	}

	return arch.CodeWritableRoots(host, tests)
}

func configureCodeRoots(deps BuildCodeDeps, roots []architecture.WritableRoot) {
	paths, patterns := make([]string, 0, len(roots)), make([]string, 0, len(roots))
	for _, root := range roots {
		paths = append(paths, root.Path)

		pattern := root.Path
		if root.Directory {
			pattern = strings.TrimSuffix(pattern, "/") + "/**"
		}

		patterns = append(patterns, pattern)
	}

	deps.BuildCodeFixApplier.UseDeclaredWriteRoots(paths, patterns)
	deps.BuildCodeFixPromptGenerator.UseImpossibleReport(paths)
}

func codeRootsForSet(
	roots []architecture.WritableRoot, services []architecture.Service, failures []*testrunner.FailingTest,
) []architecture.WritableRoot {
	names := make(map[string]bool)

	for _, failure := range failures {
		if failure.Service == "" {
			return roots
		}

		names[failure.Service] = true
	}

	selected := make([]architecture.WritableRoot, 0)

	for _, root := range roots {
		for _, service := range services {
			if names[service.Name] && (root.Path == service.Path || slices.Contains(service.Writable, root.Path)) {
				selected = append(selected, root)

				break
			}
		}
	}

	return selected
}
