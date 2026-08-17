package scenariogen

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/registry"
)

// FrameworkGoTest is the one framework this package renders for,
// because bddgo — the thing a generated file calls — is Go.
const FrameworkGoTest = "go-test"

// hasRenderer reports whether a suite's declared framework has a file
// shape this package can render. A host suite on jest or playwright is
// refused by name rather than skipped.
func hasRenderer(framework string) bool {
	return framework == FrameworkGoTest
}

// PlannedStep is one Given/When/Then line as the file will state it.
type PlannedStep struct {
	// Method is the Run method the file calls: Given, When, Then, And
	// or But.
	Method string
	// Literal is the step's text as a Go string literal, quotes
	// included.
	Literal string
}

// PlannedScenario is one scenario's generated test function.
type PlannedScenario struct {
	ID          string
	Description string
	FuncName    string
	Steps       []PlannedStep
}

// PlannedFile is one generated file and everything it needs to render.
type PlannedFile struct {
	// Path is repo-relative, as the scenarios' `path:` states it.
	Path string
	// Package is the file's package clause, derived from the suite name.
	Package string
	// ScenariosPkg is the import path of the suite's scenarios shim.
	ScenariosPkg string
	// RegistryPath is quoted into the header so a reader of a generated
	// file knows which document to edit.
	RegistryPath string
	// Scenarios are in id order.
	Scenarios []PlannedScenario
}

// Plan is every file one registry renders into, in path order.
type Plan struct {
	Files []PlannedFile
}

// BuildPlan groups scenarios by `path:`, resolves each one's owning
// suite, and refuses the whole registry if any scenario cannot be
// generated.
//
// The whole registry, not the first bad scenario's file: these mistakes
// arrive in batches — a renamed suite, a moved directory — and a
// generator that wrote nine files and then refused the tenth would leave
// a tree that is neither the old shape nor the new one.
func BuildPlan(
	scenarios []*registry.RegistryScenario,
	arch *architecture.Architecture,
	repoRoot, registryPath string,
) (*Plan, error) {
	err := checkIdentifiers(scenarios)
	if err != nil {
		return nil, err
	}

	grouped := map[string][]*registry.RegistryScenario{}
	order := []string{}

	for _, scenario := range scenarios {
		suite, suiteErr := suiteFor(scenario, arch)
		if suiteErr != nil {
			return nil, suiteErr
		}

		checkErr := checkPath(scenario, suite)
		if checkErr != nil {
			return nil, checkErr
		}

		if _, seen := grouped[scenario.Path]; !seen {
			order = append(order, scenario.Path)
		}

		grouped[scenario.Path] = append(grouped[scenario.Path], scenario)
	}

	sort.Strings(order)

	plan := &Plan{Files: make([]PlannedFile, 0, len(order))}

	for _, filePath := range order {
		file, fileErr := planFile(filePath, grouped[filePath], arch, repoRoot, registryPath)
		if fileErr != nil {
			return nil, fileErr
		}

		plan.Files = append(plan.Files, file)
	}

	err = checkPackageClauses(plan, repoRoot)
	if err != nil {
		return nil, err
	}

	return plan, nil
}

// checkPackageClauses checks each target directory once.
//
// Once per DIRECTORY rather than once per file: the check reads and
// parses every `_test.go` beside the target, and a suite whose scenarios
// group into six files would otherwise re-read the same tree — including
// the largest generated files in it — six times.
func checkPackageClauses(plan *Plan, repoRoot string) error {
	checked := map[string]bool{}

	for _, file := range plan.Files {
		dir := path.Dir(file.Path)
		if checked[dir] {
			continue
		}

		checked[dir] = true

		err := checkPackageClause(filepath.Join(repoRoot, filepath.FromSlash(dir)), file.Package)
		if err != nil {
			return err
		}
	}

	return nil
}

// planFile turns the scenarios sharing one path into a renderable file.
func planFile(
	filePath string,
	scenarios []*registry.RegistryScenario,
	arch *architecture.Architecture,
	repoRoot, registryPath string,
) (PlannedFile, error) {
	suite, err := suiteFor(scenarios[0], arch)
	if err != nil {
		return PlannedFile{}, err
	}

	for _, scenario := range scenarios[1:] {
		if scenario.Service != scenarios[0].Service {
			return PlannedFile{}, fmt.Errorf("%s: %s and %s: %w (%q vs %q)",
				filePath, scenarios[0].ID, scenario.ID, ErrPathCrossSuite,
				scenarios[0].Service, scenario.Service)
		}
	}

	scenariosPkg, err := scenariosImportPath(repoRoot, suite.Path)
	if err != nil {
		return PlannedFile{}, fmt.Errorf("%s: %w", filePath, err)
	}

	pkg, err := packageName(suite.Name)
	if err != nil {
		return PlannedFile{}, fmt.Errorf("%s: %w", filePath, err)
	}

	// Sorted by id, not by document order: a YAML map decode loses
	// source order entirely, and id order is what the walk and the run
	// report already use.
	sorted := append([]*registry.RegistryScenario(nil), scenarios...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	planned := make([]PlannedScenario, 0, len(sorted))

	for _, scenario := range sorted {
		one, planErr := planScenario(scenario)
		if planErr != nil {
			return PlannedFile{}, planErr
		}

		planned = append(planned, one)
	}

	return PlannedFile{
		Path:         filePath,
		Package:      pkg,
		ScenariosPkg: scenariosPkg,
		RegistryPath: headerRegistryPath(repoRoot, registryPath),
		Scenarios:    planned,
	}, nil
}

// headerRegistryPath is the registry path as the generated header quotes
// it: repo-relative, slash-separated, cleaned.
//
// Relativised rather than quoted as given, because these bytes are
// compared. `--requirements $(pwd)/docs/scenarios.yaml --fix` would
// otherwise bake one machine's absolute path into every generated file,
// and every later run — including one on the same machine without the
// flag — would report the whole tree as drifted. Cleaned for the same
// reason at smaller scale: the host config conventionally writes
// "./docs/…", and a header quoting the leading "./" reads like a shell
// argument rather than a document.
func headerRegistryPath(repoRoot, registryPath string) string {
	if filepath.IsAbs(registryPath) {
		relative, err := filepath.Rel(repoRoot, registryPath)
		if err == nil && !strings.HasPrefix(filepath.ToSlash(relative), "../") {
			registryPath = relative
		}
	}

	return path.Clean(filepath.ToSlash(registryPath))
}

func planScenario(scenario *registry.RegistryScenario) (PlannedScenario, error) {
	if len(scenario.Statements) == 0 {
		return PlannedScenario{}, fmt.Errorf("%s: %w", scenario.ID, ErrScenarioHasNoSteps)
	}

	steps := make([]PlannedStep, 0, len(scenario.Statements))
	for _, statement := range scenario.Statements {
		steps = append(steps, PlannedStep{
			Method:  statement.Keyword,
			Literal: literal(statement.Text),
		})
	}

	return PlannedScenario{
		ID:          scenario.ID,
		Description: collapseSpace(scenario.Description),
		FuncName:    funcName(scenario.ID),
		Steps:       steps,
	}, nil
}

// suiteFor resolves the one suite whose `service:` the scenario names.
func suiteFor(
	scenario *registry.RegistryScenario,
	arch *architecture.Architecture,
) (architecture.Suite, error) {
	for _, suite := range arch.Suites {
		if suite.Service != scenario.Service {
			continue
		}

		if !hasRenderer(suite.Framework) {
			return architecture.Suite{}, fmt.Errorf("%s: suite %s: %w: %q",
				scenario.ID, suite.Label(), ErrNoGeneratorForFramework, suite.Framework)
		}

		return suite, nil
	}

	return architecture.Suite{}, fmt.Errorf("%s: service %q: %w",
		scenario.ID, scenario.Service, ErrNoSuiteForService)
}

// checkPath refuses everything about a `path:` that the schema cannot
// see and that would make the generated file wrong rather than missing.
//
// The target is the path as WRITTEN, not a trimmed copy: validating one
// string and then writing another is how `"…_test.go "` passes every
// check here and lands on disk with a trailing space, as a file `go
// test` ignores and Verify reports as drifted forever.
func checkPath(scenario *registry.RegistryScenario, suite architecture.Suite) error {
	target := scenario.Path

	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("%s: %w", scenario.ID, ErrNoPath)
	}

	if target != strings.TrimSpace(target) {
		return fmt.Errorf("%s: %q: %w (surrounding whitespace)",
			scenario.ID, target, ErrPathNotRelative)
	}

	if path.IsAbs(target) || target != path.Clean(target) || strings.HasPrefix(target, "..") {
		return fmt.Errorf("%s: %q: %w", scenario.ID, target, ErrPathNotRelative)
	}

	if !strings.HasSuffix(target, "_test.go") {
		return fmt.Errorf("%s: %q: %w", scenario.ID, target, ErrPathNotTestFile)
	}

	suiteRoot := path.Clean(filepath.ToSlash(suite.Path))

	// Before the suite-root check, not after: a path under steps/ fails
	// both, and the reader is much better served by the one that says
	// which tree they landed in than by "not directly in the suite's
	// directory".
	if strings.HasPrefix(target, suiteRoot+"/steps/") {
		return fmt.Errorf("%s: %q: %w", scenario.ID, target, ErrPathInStepsTree)
	}

	if path.Dir(target) != suiteRoot {
		return fmt.Errorf("%s: %q: %w (suite %s declares %q)",
			scenario.ID, target, ErrPathNotInSuiteRoot, suite.Label(), suiteRoot)
	}

	return nil
}

// checkIdentifiers refuses a registry whose ids do not render runnable,
// distinct Go test names. Checked across the whole registry rather than
// per file: two colliding ids in different files still make
// `-run TestE2E001` ambiguous.
func checkIdentifiers(scenarios []*registry.RegistryScenario) error {
	seen := make(map[string]string, len(scenarios))

	for _, scenario := range scenarios {
		name := funcName(scenario.ID)

		err := checkRunnableTestName(scenario.ID, name)
		if err != nil {
			return err
		}

		previous, clash := seen[name]
		if clash {
			return fmt.Errorf("%s and %s: %w (%s)",
				previous, scenario.ID, ErrIdentifierCollision, name)
		}

		seen[name] = scenario.ID
	}

	return nil
}

// checkRunnableTestName refuses an id whose function `go test` would
// compile and never run.
//
// The rule is testing.isTest's: Test followed by anything that is NOT a
// lower-case letter. So `E2E-001` is fine and `int-900` is not — it
// renders Testint900, which compiles, is skipped in silence, and still
// satisfies every coverage check, because a call site exists and only
// the runner knows the function is not a test. That is the worst
// available failure: a scenario that reports as covered and never runs.
func checkRunnableTestName(scenarioID, name string) error {
	rest := strings.TrimPrefix(name, testPrefix)

	if rest == "" {
		return fmt.Errorf("%q: %w: it renders %q, which names no scenario",
			scenarioID, ErrUnrunnableTestName, name)
	}

	first, _ := utf8.DecodeRuneInString(rest)
	if unicode.IsLower(first) {
		return fmt.Errorf("%q: %w: it renders %q, and `go test` does not run a "+
			"Test name followed by a lower-case letter — start the id with an "+
			"upper-case letter or a digit",
			scenarioID, ErrUnrunnableTestName, name)
	}

	return nil
}

// testPrefix is what `go test` looks for at the front of a test name.
const testPrefix = "Test"

// funcName renders a scenario id as a Go test function name.
func funcName(scenarioID string) string {
	var name strings.Builder

	name.WriteString(testPrefix)

	for _, char := range scenarioID {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			name.WriteRune(char)
		}
	}

	return name.String()
}

// packageName derives the generated file's package from the suite name:
// `bdd-cli` becomes `bddcli_test`.
func packageName(suiteName string) (string, error) {
	var name strings.Builder

	for _, char := range strings.ToLower(suiteName) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			name.WriteRune(char)
		}
	}

	derived := name.String()

	// A suite named `2e2` would derive `2e2_test`, which is not a Go
	// identifier at all — caught here, naming the suite, rather than
	// later as a parse error from format.Source that names nothing.
	if derived == "" || unicode.IsDigit(rune(derived[0])) {
		return "", fmt.Errorf("suite %q: %w (derived %q)",
			suiteName, ErrPackageNameInvalid, derived+"_test")
	}

	return derived + "_test", nil
}

// checkPackageClause refuses a target directory that already declares a
// different package.
//
// Renaming a suite in the architectural spec silently re-derives the
// package name; without this the regeneration writes `package cli_test`
// into a directory whose hand-written main_test.go still says
// `bddcli_test`, and the suite simply stops compiling with nothing
// pointing at the rename.
//
// Two things it must NOT refuse. A file already carrying the generated
// marker is the generator's own previous output, and refusing it would
// make a suite rename permanently unfixable — `--fix` could never write
// the new package because the old one is still on disk. And Go allows an
// internal test package beside the external one, so a legal `package
// bddcli` file in a `bddcli_test` directory is not a mismatch.
func checkPackageClause(dir, want string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}

	fset := token.NewFileSet()

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		err = checkOnePackageClause(fset, filepath.Join(dir, entry.Name()), want)
		if err != nil {
			return err
		}
	}

	return nil
}

// checkOnePackageClause checks one hand-written test file's package
// clause against what the suite name derives.
func checkOnePackageClause(fset *token.FileSet, full, want string) error {
	source, err := os.ReadFile(full)
	if err != nil {
		return fmt.Errorf("read %s: %w", full, err)
	}

	if hasGeneratedMarker(source) {
		return nil
	}

	// A file that will not parse is left alone rather than reported: this
	// check is about a package NAME, and a file Go cannot read at all is
	// already a failure the compiler states better than a generator can.
	file, parseErr := parser.ParseFile(fset, full, source, parser.PackageClauseOnly)
	if parseErr == nil && file.Name.Name != want &&
		file.Name.Name != strings.TrimSuffix(want, "_"+testPkgSuffix) {
		return fmt.Errorf("%s: %w: the directory declares %q, the suite name derives %q",
			full, ErrPackageClauseMismatch, file.Name.Name, want)
	}

	return nil
}

// testPkgSuffix is what Go appends to an external test package's name.
const testPkgSuffix = "test"

// collapseSpace folds a description onto one line so it can sit in a
// single Go comment. A YAML folded scalar arrives with newlines in it.
func collapseSpace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
