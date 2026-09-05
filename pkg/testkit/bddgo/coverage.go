package bddgo

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// generatedMarker is Go's own convention for a machine-written file, and
// what `.golangci.yaml`'s generated-file exclusion keys on. Matched
// against a trimmed line (CRLF-safe); the engine's scenariogen keeps a copy in sync.
var generatedMarker = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)

// ErrNotGenerated signals a file holding a scenario call but carrying no
// generated marker — a generated test someone started hand-editing.
var ErrNotGenerated = errors.New("file declares a scenario but is not marked generated")

// ScenarioCall is one `scenarios.New(t, "<ID>")` site, as found by
// parsing a package's source.
type ScenarioCall struct {
	// File is repo-relative when CheckCoverage found it, and as given to
	// ScanScenarioCalls otherwise.
	File string
	// Func is the top-level Test function the call sits in.
	Func string
	ID   string
	Line int
}

// ScanScenarioCalls parses every .go file in dir and returns each
// two-argument `<pkg>.New(<ident>, "<literal>")` call whose enclosing
// function is a top-level Test function.
func ScanScenarioCalls(dir string) ([]ScenarioCall, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}

	fset := token.NewFileSet()

	var calls []ScenarioCall

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		path := filepath.Join(dir, entry.Name())

		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			// An unparseable file is an error, not a file to skip: the
			// scenarios it declares would otherwise vanish from the
			// coverage answer and the gap would read as "none".
			return nil, fmt.Errorf("parse %s: %w", path, parseErr)
		}

		calls = append(calls, scenarioCallsIn(fset, file, path)...)
	}

	sort.Slice(calls, func(i, j int) bool {
		if calls[i].File != calls[j].File {
			return calls[i].File < calls[j].File
		}

		return calls[i].Line < calls[j].Line
	})

	return calls, nil
}

// scenarioCallsIn collects the scenario calls in one parsed file.
func scenarioCallsIn(fset *token.FileSet, file *ast.File, path string) []ScenarioCall {
	var calls []ScenarioCall

	for _, decl := range file.Decls {
		function, isFunc := decl.(*ast.FuncDecl)
		if !isFunc || function.Recv != nil || function.Body == nil ||
			!strings.HasPrefix(function.Name.Name, "Test") {
			continue
		}

		ast.Inspect(function.Body, func(node ast.Node) bool {
			scenarioID, line, found := scenarioCallID(fset, node)
			if !found {
				return true
			}

			calls = append(calls, ScenarioCall{
				File: path, Func: function.Name.Name, ID: scenarioID, Line: line,
			})

			return true
		})
	}

	return calls
}

// scenarioCallID reports the scenario id of a `<pkg>.New(t, "<id>")`
// call, matching on shape rather than on the package's name — a suite is
// free to import its shim under any alias.
func scenarioCallID(fset *token.FileSet, node ast.Node) (string, int, bool) {
	call, isCall := node.(*ast.CallExpr)
	if !isCall || len(call.Args) != 2 {
		return "", 0, false
	}

	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector || selector.Sel.Name != "New" {
		return "", 0, false
	}

	if _, isIdent := selector.X.(*ast.Ident); !isIdent {
		return "", 0, false
	}

	if _, isIdent := call.Args[0].(*ast.Ident); !isIdent {
		return "", 0, false
	}

	literal, isLiteral := call.Args[1].(*ast.BasicLit)
	if !isLiteral || literal.Kind != token.STRING {
		return "", 0, false
	}

	scenarioID, err := strconv.Unquote(literal.Value)
	if err != nil {
		return "", 0, false
	}

	return scenarioID, fset.Position(call.Pos()).Line, true
}

// CheckCoverage refuses a suite whose registry and generated test files
// disagree, in either direction: a scenario with no test executes
// nothing, and a test naming an unknown scenario fails on its own.
func (s *Suite[S]) CheckCoverage(t *testing.T, pkgDir, repoRoot string) {
	t.Helper()

	if s.err != nil {
		t.Fatalf("%v", s.err)
	}

	owned, err := s.Owned()
	if err != nil {
		t.Fatalf("%v", err)
	}

	if len(owned) == 0 {
		t.Fatalf("%s: suite %q (service %q): %v",
			s.opts.Registry, s.spec.Name, s.spec.Service, ErrNoScenariosForSuite)
	}

	calls, err := ScanScenarioCalls(pkgDir)
	if err != nil {
		t.Fatalf("%v", err)
	}

	calls = relativeCalls(t, calls, repoRoot)
	byID := indexCalls(t, calls)

	checkEveryScenarioHasATest(t, owned, byID)
	checkEveryTestHasAScenario(t, s, calls)
	checkGeneratedMarkers(t, calls, repoRoot)
}

// relativeCalls rewrites each call's file to a repo-relative path, so it
// can be compared with the registry's `path:`.
func relativeCalls(t *testing.T, calls []ScenarioCall, repoRoot string) []ScenarioCall {
	t.Helper()

	out := make([]ScenarioCall, 0, len(calls))

	for _, call := range calls {
		absolute, err := filepath.Abs(call.File)
		if err != nil {
			t.Fatalf("resolve %s: %v", call.File, err)
		}

		relative, err := filepath.Rel(repoRoot, absolute)
		if err != nil {
			t.Fatalf("relativize %s: %v", absolute, err)
		}

		call.File = filepath.ToSlash(relative)
		out = append(out, call)
	}

	return out
}

// indexCalls keys the calls by scenario id, reporting any id declared
// twice — two tests running one scenario, where the second is dead
// weight nobody reads and the first is what actually gates.
func indexCalls(t *testing.T, calls []ScenarioCall) map[string]ScenarioCall {
	t.Helper()

	byID := make(map[string]ScenarioCall, len(calls))

	for _, call := range calls {
		previous, seen := byID[call.ID]
		if seen {
			t.Errorf("%s is declared by two tests: %s:%d %s and %s:%d %s",
				call.ID, previous.File, previous.Line, previous.Func, call.File, call.Line, call.Func)

			continue
		}

		byID[call.ID] = call
	}

	return byID
}

func checkEveryScenarioHasATest(t *testing.T, owned []Scenario, byID map[string]ScenarioCall) {
	t.Helper()

	for _, scenario := range owned {
		call, found := byID[scenario.ID]
		if !found {
			t.Errorf("%s (%s) has no generated test; expected one in %s\n  regenerate with: %s",
				scenario.ID, scenario.Description, scenario.Path, regenerate)

			continue
		}

		if scenario.Path != "" && call.File != scenario.Path {
			t.Errorf("%s is declared in %s but the registry says %s\n  regenerate with: %s",
				scenario.ID, call.File, scenario.Path, regenerate)
		}
	}
}

func checkEveryTestHasAScenario[S any](t *testing.T, suite *Suite[S], calls []ScenarioCall) {
	t.Helper()

	for _, call := range calls {
		scenario, found, err := suite.Lookup(call.ID)
		if err != nil {
			t.Fatalf("%v", err)
		}

		switch {
		case !found:
			t.Errorf("%s:%d %s declares %s, which %v\n  regenerate with: %s",
				call.File, call.Line, call.Func, call.ID, ErrScenarioNotInRegistry, regenerate)
		case !suite.spec.Owns(scenario):
			t.Errorf("%s:%d %s declares %s, but %v: it names service %q\n  regenerate with: %s",
				call.File, call.Line, call.Func, call.ID, ErrScenarioNotOwned, scenario.Service, regenerate)
		}
	}
}

// checkGeneratedMarkers refuses a file that declares a scenario without
// saying it was generated: the marker is what tells the next
// regeneration the file is safe to overwrite.
func checkGeneratedMarkers(t *testing.T, calls []ScenarioCall, repoRoot string) {
	t.Helper()

	checked := map[string]bool{}

	for _, call := range calls {
		if checked[call.File] {
			continue
		}

		checked[call.File] = true

		data, err := disk.Read(filepath.Join(repoRoot, call.File))
		if err != nil {
			t.Errorf("read %s: %v", call.File, err)

			continue
		}

		if !hasGeneratedMarker(data) {
			t.Errorf("%s: %v\n  regenerate with: %s", call.File, ErrNotGenerated, regenerate)
		}
	}
}

// hasGeneratedMarker reports whether the marker appears in the file's
// leading comment block, which is the only place Go's convention puts it.
func hasGeneratedMarker(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			if generatedMarker.MatchString(trimmed) {
				return true
			}

			continue
		}

		return false
	}

	return false
}

// CheckFixtureTrees refuses a run in which the registry and a fixtures
// directory disagree: a scenario naming a missing tree fails loudly, but
// a TREE with no scenario just stops running — silently, forever.
func (s *Suite[S]) CheckFixtureTrees(t *testing.T, dir string, name func(Scenario) string) {
	t.Helper()

	owned, err := s.Owned()
	if err != nil {
		t.Fatalf("%v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}

	claimed := checkTreesClaimedOnce(t, owned, name)

	var orphans []string

	for _, entry := range entries {
		if !entry.IsDir() || claimed[entry.Name()] {
			continue
		}

		orphans = append(orphans, entry.Name())
	}

	if len(orphans) == 0 {
		return
	}

	sort.Strings(orphans)
	t.Fatalf("%d fixture tree(s) no scenario names, so nothing runs them: %s\n"+
		"add a scenario to %s for each, or delete the tree",
		len(orphans), strings.Join(orphans, ", "), s.opts.Registry)
}

// checkTreesClaimedOnce indexes the trees the owned scenarios name and
// refuses a tree two of them name: the harness keys a run dir on tree
// name and clears it per run, so a duplicate deletes the first's record.
func checkTreesClaimedOnce(t *testing.T, owned []Scenario, name func(Scenario) string) map[string]bool {
	t.Helper()

	claimed := make(map[string]bool, len(owned))
	byTree := make(map[string]string, len(owned))

	for _, scenario := range owned {
		tree := name(scenario)

		previous, seen := byTree[tree]
		if seen {
			t.Errorf("%s and %s both name the fixture tree %q; "+
				"the second run would delete the first's record\n"+
				"  give one of them its own tree",
				previous, scenario.ID, tree)

			continue
		}

		byTree[tree] = scenario.ID
		claimed[tree] = true
	}

	return claimed
}

// Unbound resolves every owned scenario without running anything and
// returns the steps no definition matches, keyed by scenario id. Empty
// means the suite covers its half of the registry.
func (s *Suite[S]) Unbound() (map[string][]Step, error) {
	if s.err != nil {
		return nil, s.err
	}

	scenarios, err := s.Owned()
	if err != nil {
		return nil, err
	}

	gaps := map[string][]Step{}

	for _, scenario := range scenarios {
		_, undefined, resolveErr := s.resolve(scenario)
		if resolveErr != nil {
			return nil, resolveErr
		}

		if len(undefined) > 0 {
			gaps[scenario.ID] = undefined
		}
	}

	return gaps, nil
}

// coverageReportEnv names the DIRECTORY the suites write their reports
// into, one file each — one command may start several, and a shared path
// keeps only the last writer. Duplicated in the engine: a wire protocol.
const coverageReportEnv = "TRUEBDD_COVERAGE_REPORT_DIR"

// coverageSchema is the report format's version. The engine refuses a
// number it does not know rather than reading a future shape with
// today's meanings.
const coverageSchema = 1

// coverageGap is one unbound step, as the report states it.
type coverageGap struct {
	Scenario string `json:"scenario"`
	Step     string `json:"step"`
}

// coverageAmbiguity is one step matched by more than one definition.
type coverageAmbiguity struct {
	Scenario string   `json:"scenario"`
	Step     string   `json:"step"`
	Patterns []string `json:"patterns"`
}

// coverageReport is what a suite writes when asked which steps bind.
type coverageReport struct {
	Schema int    `json:"schema"`
	Suite  string `json:"suite"`
	// Examined names every scenario this suite actually resolved. Without
	// it, an empty Unbound is ambiguous between "every step binds" and
	// "died before it looked" — the engine would read a crash as full coverage.
	Examined  []string            `json:"examined"`
	Unbound   []coverageGap       `json:"unbound"`
	Ambiguous []coverageAmbiguity `json:"ambiguous"`
	// Failure is set when the suite could not answer at all — an
	// uncompilable step pattern, a registry that would not load. Distinct
	// from Ambiguous, which is a real finding about a real step.
	Failure string `json:"failure,omitempty"`
}

// ReportStepCoverage fails for every registry step that binds to no step
// definition, and — when TRUEBDD_COVERAGE_REPORT is set — writes the
// same answer as JSON for `build tests` to read.
func (s *Suite[S]) ReportStepCoverage(t *testing.T) {
	t.Helper()

	report := coverageReport{Schema: coverageSchema, Suite: s.spec.Name}

	gaps, err := s.Unbound()
	if err != nil {
		// Populated field by field, not as one formatted string: the
		// engine renders the scenario id and patterns into its own
		// refusal text, and blank fields would double the message.
		var ambiguous *AmbiguousStepError

		if errors.As(err, &ambiguous) {
			report.Ambiguous = append(report.Ambiguous, coverageAmbiguity{
				Scenario: ambiguous.Scenario,
				Step:     ambiguous.Step,
				Patterns: ambiguous.Patterns,
			})
		} else {
			report.Failure = err.Error()
		}

		writeCoverageReport(t, report)
		t.Fatalf("%v", err)
	}

	owned, err := s.Owned()
	if err != nil {
		report.Failure = err.Error()

		writeCoverageReport(t, report)
		t.Fatalf("%v", err)
	}

	for _, scenario := range owned {
		report.Examined = append(report.Examined, scenario.ID)

		for _, step := range gaps[scenario.ID] {
			report.Unbound = append(report.Unbound,
				coverageGap{Scenario: scenario.ID, Step: step.String()})
		}
	}

	writeCoverageReport(t, report)

	for _, scenario := range owned {
		if len(gaps[scenario.ID]) == 0 {
			continue
		}

		t.Errorf("%s", undefinedReport(scenario, gaps[scenario.ID]))
	}
}

// writeCoverageReport publishes the report when the engine asked for
// one. Absent env means a person is running the test, and a person reads
// the failures. Named after the suite, so two suites never collide.
func writeCoverageReport(t *testing.T, report coverageReport) {
	t.Helper()

	dir := os.Getenv(coverageReportEnv)
	if dir == "" {
		return
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("render the coverage report: %v", err)
	}

	// Created, not assumed: the runner handing the path over need not be
	// this engine.
	err = disk.Dir(dir, disk.Shared)
	if err != nil {
		t.Fatalf("create the coverage report directory: %v", err)
	}

	err = disk.Write(filepath.Join(dir, report.Suite+".json"), data, disk.Shared)
	if err != nil {
		t.Fatalf("write the coverage report: %v", err)
	}
}
