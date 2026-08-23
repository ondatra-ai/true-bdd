package steps

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/tests/libraries/bddgo"
)

// assertScenarioStepsMatched pins that a `build tests --fix` run left a
// named scenario EXECUTABLE: every step resolves to exactly one pattern
// parsed from the fix loop's Go source — zero is unbound, more than one is ambiguous.
func assertScenarioStepsMatched(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	scenarioID := args[0]
	registryRel := args[1]
	stepsDirRel := args[2]

	registryPath, err := state.containedPath(registryRel)
	if err != nil {
		return err
	}

	scenarios, err := bddgo.LoadRegistry(registryPath)
	if err != nil {
		return state.fail("reading registry %q: %w", registryRel, err)
	}

	scenario, found := findScenario(scenarios, scenarioID)
	if !found {
		return state.fail("registry %q declares no scenario %q", registryRel, scenarioID)
	}

	stepsDir, err := state.containedDir(stepsDirRel)
	if err != nil {
		return err
	}

	patterns, err := extractStepPatterns(stepsDir)
	if err != nil {
		return state.fail("reading step definitions under %q: %w", stepsDirRel, err)
	}

	unresolved := unresolvedSteps(scenario, patterns)
	if len(unresolved) > 0 {
		return state.fail(
			"expected every step of scenario %q in %q to resolve to exactly one definition under %q, but: %s",
			scenarioID, registryRel, stepsDirRel, strings.Join(unresolved, "; "))
	}

	return nil
}

// unresolvedSteps reports steps that fail to resolve to exactly one
// pattern (zero: unbound; many: ambiguous). A model-run (`llm:`/`judge:`)
// step binds by construction, so it is skipped rather than needing a pattern.
func unresolvedSteps(scenario bddgo.Scenario, patterns []*regexp.Regexp) []string {
	var unresolved []string

	for _, step := range scenario.Steps {
		if step.Mode != bddgo.ModeDeterministic {
			continue
		}

		matches := 0

		for _, pattern := range patterns {
			if pattern.MatchString(step.Text) {
				matches++
			}
		}

		switch {
		case matches == 0:
			unresolved = append(unresolved,
				fmt.Sprintf("%q resolves to no definition", step.Text))
		case matches > 1:
			unresolved = append(unresolved,
				fmt.Sprintf("%q resolves to %d definitions", step.Text, matches))
		}
	}

	return unresolved
}

// findScenario returns the scenario with the given id, and whether the
// registry declared one — an absent scenario is a different failure than
// one present with an unbound step.
func findScenario(scenarios []bddgo.Scenario, id string) (bddgo.Scenario, bool) {
	for _, scenario := range scenarios {
		if scenario.ID == id {
			return scenario, true
		}
	}

	return bddgo.Scenario{}, false
}

// extractStepPatterns reads each `.Step(…)` call's string-literal pattern
// from the Go source under dir, compiled as a regexp. Parsed, not run: the
// code lives in a separate module this suite cannot import or build.
func extractStepPatterns(dir string) ([]*regexp.Regexp, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}

	fset := token.NewFileSet()

	var patterns []*regexp.Regexp

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		file, parseErr := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, 0)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing %s: %w", entry.Name(), parseErr)
		}

		ast.Inspect(file, func(node ast.Node) bool {
			pattern, isStep := stepCallPattern(node)
			if isStep {
				patterns = append(patterns, pattern)
			}

			return true
		})
	}

	return patterns, nil
}

// stepCallPattern returns the compiled regexp of a `.Step("<pattern>", …)`
// call, and whether node was one. A non-literal or non-compiling argument
// is skipped, not an error — it may be an unrelated method also named Step.
func stepCallPattern(node ast.Node) (*regexp.Regexp, bool) {
	call, isCall := node.(*ast.CallExpr)
	if !isCall {
		return nil, false
	}

	sel, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector || sel.Sel.Name != "Step" || len(call.Args) < 1 {
		return nil, false
	}

	lit, isLiteral := call.Args[0].(*ast.BasicLit)
	if !isLiteral || lit.Kind != token.STRING {
		return nil, false
	}

	raw, err := strconv.Unquote(lit.Value)
	if err != nil {
		return nil, false
	}

	compiled, err := regexp.Compile(raw)
	if err != nil {
		return nil, false
	}

	return compiled, true
}
