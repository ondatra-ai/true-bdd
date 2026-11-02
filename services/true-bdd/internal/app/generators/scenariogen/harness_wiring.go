package scenariogen

import (
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

func writeHarnessWiring(file PlannedFile, suiteDir string, aiCLIs []string) error {
	target, err := wiringTarget(filepath.Join(suiteDir, scenariosPkgDir))
	if err != nil {
		return err
	}

	if target != "" {
		quoted := make([]string, 0, len(aiCLIs))
		for _, name := range aiCLIs {
			quoted = append(quoted, strconv.Quote(name))
		}

		source, renderErr := format.Source([]byte(fmt.Sprintf(harnessWiringSource, wiringMarker,
			strings.TrimSuffix(file.ScenariosPkg, "/"+scenariosPkgDir)+"/steps", file.Service, strings.Join(quoted, ", "))))
		if renderErr != nil {
			return fmt.Errorf("render harness Suite wiring: %w", renderErr)
		}

		err = writeWiringSource(target, source)
		if err != nil {
			return err
		}
	}

	target, err = testMainTarget(suiteDir)
	if err != nil || target == "" {
		return err
	}

	source, err := format.Source([]byte(fmt.Sprintf(testMainSource, wiringMarker, file.Package, file.ScenariosPkg)))
	if err != nil {
		return fmt.Errorf("render TestMain: %w", err)
	}

	return writeWiringSource(target, source)
}

func testMainTarget(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("read TestMain directory: %w", err)
	}

	target := ""

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		filename := filepath.Join(dir, entry.Name())

		marked, blocked, readErr := testMainOwnership(filename)
		if readErr != nil {
			return "", readErr
		}

		if blocked {
			return "", nil
		}

		if marked {
			target = filename
		}
	}

	return missingTestMainTarget(dir, target)
}

func missingTestMainTarget(dir, marked string) (string, error) {
	if marked != "" {
		return marked, nil
	}

	target := filepath.Join(dir, "main_test.go")

	_, err := os.Stat(target)
	if errors.Is(err, fs.ErrNotExist) {
		return target, nil
	}

	if err != nil {
		return "", fmt.Errorf("locate TestMain: %w", err)
	}

	return "", nil
}

func testMainOwnership(filename string) (bool, bool, error) {
	source, err := disk.Read(filename)
	if err != nil {
		return false, false, fmt.Errorf("read TestMain: %w", err)
	}

	if ownsWiring(source) {
		return true, false, nil
	}

	tree, err := parser.ParseFile(token.NewFileSet(), filename, source, 0)
	if err != nil {
		return false, false, fmt.Errorf("parse TestMain: %w", err)
	}

	for _, declaration := range tree.Decls {
		if fun, ok := declaration.(*ast.FuncDecl); ok && fun.Name.Name == "TestMain" && fun.Recv == nil {
			return false, true, nil
		}
	}

	return false, false, nil
}

const harnessWiringSource = `//go:build bdd

%s

package scenarios

import (
 "testing"

 "github.com/ondatra-ai/true-bdd/pkg/testkit"
 "github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
 %q
)

//nolint:gochecknoglobals // one suite per test binary; the door owns its lifecycle.
var suite = testkit.NewSuite(%q, []string{%s},
 func(suite *bddgo.Suite[steps.State]) { steps.Register(suite) },
 func(run *testkit.ScenarioRun) *steps.State { return &steps.State{ScenarioRun: run} })

// Main returns after the door has torn down the suite's shared resources.
func Main(m *testing.M) int { return suite.Main(m) }

// New selects the Scenario a generated test drives.
func New(t *testing.T, scenarioID string) *bddgo.Run[steps.State] {
 t.Helper()
 return suite.New(t, scenarioID)
}

// CheckCoverage compares the Registry and generated tests without harness bring-up.
func CheckCoverage(t *testing.T) {
 t.Helper()
 suite.CheckCoverage(t)
}

// CheckFixtureTrees refuses a fixture no Scenario names.
func CheckFixtureTrees(t *testing.T) {
 t.Helper()
 suite.CheckFixtureTrees(t)
}
`

const testMainSource = `//go:build bdd

%s

package %s

import (
 "os"
 "testing"

 %q
)

func TestMain(m *testing.M) {
 os.Exit(scenarios.Main(m))
}
`
