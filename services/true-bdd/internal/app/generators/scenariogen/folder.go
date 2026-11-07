package scenariogen

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
)

var errStepsMissing = errors.New("steps package is missing")
var errSuiteFolder = errors.New("suite folder disagrees with the Registry")

// RequireSteps refuses before rendering when a suite has no Steps package.
func RequireSteps(plan *Plan, root string) error {
	for _, file := range plan.Files {
		dir := filepath.Join(root, filepath.Dir(file.Path), "steps")

		entries, err := os.ReadDir(dir)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("inspect Steps package: %w", err)
		}

		found := false

		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
				found = true
			}
		}

		if !found {
			return fmt.Errorf("%w: %s; run true-bdd build steps --fix", errStepsMissing, dir)
		}
	}

	return nil
}

// CheckFolder holds the two host-wide folder checks without running a test.
func CheckFolder(scenarios []*registry.RegistryScenario, root string) error {
	byID := make(map[string]*registry.RegistryScenario)
	suites := make(map[string]bool)

	for _, scenario := range scenarios {
		byID[scenario.ID] = scenario
		suites[filepath.Dir(scenario.Path)] = true
	}

	counts := make(map[string]int)
	for suite := range suites {
		err := checkScenarioCalls(root, suite, byID, counts)
		if err != nil {
			return err
		}

		err = checkFixtures(root, suite, scenarios)
		if err != nil {
			return err
		}
	}

	for _, scenario := range scenarios {
		if counts[scenario.ID] != 1 {
			return fmt.Errorf("%w: %s has %d generated tests; expected one in %s", errSuiteFolder,
				scenario.ID, counts[scenario.ID], scenario.Path)
		}
	}

	return nil
}

func checkScenarioCalls(root, suite string, byID map[string]*registry.RegistryScenario, counts map[string]int) error {
	entries, err := os.ReadDir(filepath.Join(root, suite))
	if err != nil {
		return fmt.Errorf("read suite folder: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !goTestSource(entry.Name()) {
			continue
		}

		name := filepath.ToSlash(filepath.Join(suite, entry.Name()))

		err = checkScenarioFile(root, name, byID, counts)
		if err != nil {
			return err
		}
	}

	return nil
}

func checkScenarioFile(root, name string, byID map[string]*registry.RegistryScenario, counts map[string]int) error {
	source, err := disk.Read(filepath.Join(root, name))
	if err != nil {
		return fmt.Errorf("read suite test: %w", err)
	}

	tree, err := parser.ParseFile(token.NewFileSet(), name, source, 0)
	if err != nil {
		return fmt.Errorf("parse suite test: %w", err)
	}

	for _, scenarioID := range scenarioCalls(tree) {
		scenario := byID[scenarioID]
		if scenario == nil {
			return fmt.Errorf("%w: %s names missing Scenario %s", errSuiteFolder, name, scenarioID)
		}

		if scenario.Path != name || !hasGeneratedMarker(source) {
			return fmt.Errorf("%w: %s needs its generated test in %s", errSuiteFolder, scenarioID, scenario.Path)
		}

		counts[scenarioID]++
	}

	return nil
}

func scenarioCalls(tree *ast.File) []string {
	var ids []string

	for _, declaration := range tree.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil || !strings.HasPrefix(function.Name.Name, "Test") {
			continue
		}

		ast.Inspect(function.Body, func(node ast.Node) bool {
			if id := scenarioCallID(node); id != "" {
				ids = append(ids, id)
			}

			return true
		})
	}

	return ids
}

func checkFixtures(root, suite string, scenarios []*registry.RegistryScenario) error {
	dir := filepath.Join(root, suite, "fixtures")

	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("read fixtures: %w", err)
	}

	claimed := make(map[string]bool)

	for _, scenario := range scenarios {
		if filepath.Dir(scenario.Path) == suite {
			for _, name := range registry.NamedFixtures(scenario, dir) {
				claimed[name] = true
			}
		}
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if !claimed[entry.Name()] {
			return fmt.Errorf("%w: fixture tree %s has no Scenario", errSuiteFolder, filepath.Join(dir, entry.Name()))
		}
	}

	return nil
}

func scenarioCallID(node ast.Node) string {
	call, isCall := node.(*ast.CallExpr)
	if !isCall || len(call.Args) != 2 {
		return ""
	}

	fun, isIdentifier := call.Fun.(*ast.Ident)
	if !isIdentifier || fun.Name != "New" {
		return ""
	}

	literal, isLiteral := call.Args[1].(*ast.BasicLit)
	if !isLiteral || literal.Kind != token.STRING {
		return ""
	}

	id, err := strconv.Unquote(literal.Value)
	if err != nil {
		return ""
	}

	return id
}
