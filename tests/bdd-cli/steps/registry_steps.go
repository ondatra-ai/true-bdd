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

// assertScenarioStepsMatched pins that a `build tests --fix` run left an
// inner project's named scenario EXECUTABLE: every one of its steps
// resolves to exactly one step definition in the suite's steps package.
// This is the outcome the whole fix loop exists to reach — the loop is
// done not when *a* file appeared but when the scenario the file was
// written for now binds — and it is a claim no file-effect assertion can
// make, because a created .go file could register the wrong pattern, or
// none.
//
// It mirrors bddgo's own resolution semantics against artefacts on disk:
// the inner registry is parsed with bddgo.LoadRegistry (so a model-run
// `llm:`/`judge:` step is bound by construction, exactly as bddgo binds
// it), and the step definitions are read out of the Go source the fix
// loop wrote — every `.Step(<pattern>, …)` call's regexp literal. A step
// passes when exactly one of those patterns matches its text; zero is
// unbound, more than one is ambiguous, and both are reported.
//
// The scenario id, the registry path and the steps directory are all
// capture groups, both paths relative to the run's tmpdir, so one
// definition serves every scenario naming a different target.
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

// unresolvedSteps reports every step of a scenario that does not resolve
// to exactly one of the given patterns, saying which way it failed: zero is
// unbound, more than one is ambiguous, and either makes the scenario
// non-executable.
//
// A model-run step binds by construction — no regexp settles an
// `llm:`/`judge:` step and none should, so bddgo resolves it the moment it
// is read, and this assertion must not demand a pattern for it either.
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
// registry declared one at all — a scenario the run was supposed to make
// executable but that is absent is a different failure than one present
// with an unbound step.
func findScenario(scenarios []bddgo.Scenario, id string) (bddgo.Scenario, bool) {
	for _, scenario := range scenarios {
		if scenario.ID == id {
			return scenario, true
		}
	}

	return bddgo.Scenario{}, false
}

// extractStepPatterns reads every registered step pattern out of the Go
// source under dir: the first string-literal argument of each `.Step(…)`
// call, compiled as a regexp. It parses the source rather than running it
// because the code lives in a separate module inside the run's tmpdir
// that this suite cannot import or build — but the patterns a scenario
// resolves against are exactly these literals, and go/parser reads them
// without a toolchain. A file that does not parse is a fix that did not
// produce valid Go, and its error is surfaced rather than swallowed.
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
// call node, and whether node was one. A call whose first argument is not
// a string literal, or whose literal does not unquote or compile, is not
// one — it is skipped, not an error, because a helper named Step that took
// a computed pattern would be someone else's method, not a registration.
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
