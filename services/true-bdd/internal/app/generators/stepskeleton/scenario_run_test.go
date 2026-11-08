package stepskeleton_test

import (
	"fmt"
	"go/format"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

func TestScenarioRunEmbeddingPreservesDefinitionOwnedDeclarations(t *testing.T) {
	t.Parallel()

	for _, testCase := range scenarioRunImportCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(t.TempDir(), "steps")
			file := filepath.Join(dir, "definitions.go")
			source := fmt.Sprintf(scenarioRunOriginalState, testCase.imports)

			err := disk.Write(file, []byte(source), disk.Shared)
			if err != nil {
				t.Fatal(err)
			}

			err = stepskeleton.EnsureScenarioRun(dir)
			if err != nil {
				t.Fatal(err)
			}

			want, err := format.Source(fmt.Appendf(nil, scenarioRunEmbeddedState, testCase.wantImports, testCase.field))
			if err != nil {
				t.Fatal(err)
			}

			got, err := disk.Read(file)
			if err != nil {
				t.Fatal(err)
			}

			if string(got) != string(want) {
				t.Fatalf("Scenario run embedding changed declarations or imports:\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func scenarioRunImportCases() []struct{ name, imports, wantImports, field string } {
	return []struct{ name, imports, wantImports, field string }{
		{"grouped imports", "import (\n\"fmt\"\n\"strings\"\n)",
			"import (\n\"github.com/ondatra-ai/true-bdd/pkg/testkit\"\n\n\"fmt\"\n\"strings\"\n)",
			plainScenarioRunField},
		{"single import", "import \"fmt\"",
			"import (\n\"fmt\"\n\"github.com/ondatra-ai/true-bdd/pkg/testkit\"\n)", plainScenarioRunField},
		{"no imports", "", "import \"github.com/ondatra-ai/true-bdd/pkg/testkit\"", plainScenarioRunField},
		{"aliased testkit", "import harness \"github.com/ondatra-ai/true-bdd/pkg/testkit\"",
			"import harness \"github.com/ondatra-ai/true-bdd/pkg/testkit\"", "*harness.ScenarioRun"},
	}
}

const plainScenarioRunField = "*testkit.ScenarioRun"

func TestScenarioRunRepeatedCallLeavesEveryFileUnchanged(t *testing.T) {
	t.Parallel()

	for _, testCase := range scenarioRunImportCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(t.TempDir(), "steps")
			for name, source := range map[string]string{
				"definitions.go": fmt.Sprintf(scenarioRunOriginalState, testCase.imports),
				"other.go":       "package steps\nfunc other( )string{return \"untouched\"}\n",
			} {
				err := disk.Write(filepath.Join(dir, name), []byte(source), disk.Shared)
				if err != nil {
					t.Fatal(err)
				}
			}

			err := stepskeleton.EnsureScenarioRun(dir)
			if err != nil {
				t.Fatal(err)
			}

			before := scenarioRunFiles(t, dir)

			err = stepskeleton.EnsureScenarioRun(dir)
			if err != nil {
				t.Fatal(err)
			}

			if after := scenarioRunFiles(t, dir); !maps.Equal(before, after) {
				t.Fatalf("repeated call changed files: got %q, want %q", after, before)
			}
		})
	}
}

func TestScenarioRunExistingFieldLeavesSourceBytesUnchanged(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct{ name, source string }{
		{"plain import", `package steps
import "github.com/ondatra-ai/true-bdd/pkg/testkit"
type State struct{ *testkit.ScenarioRun; Name   string }
func(state *State)Reset( ){state.Name=""}
`},
		{"aliased import", `package steps
import harness "github.com/ondatra-ai/true-bdd/pkg/testkit"
type State struct{ *harness.ScenarioRun; Name   string }
func(state *State)Reset( ){state.Name=""}
`},
		{"dot import", `package steps
import . "github.com/ondatra-ai/true-bdd/pkg/testkit"
type State struct{ *ScenarioRun; Name   string }
func(state *State)Reset( ){state.Name=""}
`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(t.TempDir(), "steps")

			err := disk.Write(filepath.Join(dir, "definitions.go"), []byte(testCase.source), disk.Shared)
			if err != nil {
				t.Fatal(err)
			}

			err = stepskeleton.EnsureScenarioRun(dir)
			if err != nil {
				t.Fatal(err)
			}

			want := map[string]string{"definitions.go": testCase.source}
			if got := scenarioRunFiles(t, dir); !maps.Equal(want, got) {
				t.Fatalf("existing Scenario run field changed files: got %q, want %q", got, want)
			}
		})
	}
}

func scenarioRunFiles(t *testing.T, dir string) map[string]string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	files := make(map[string]string, len(entries))

	for _, entry := range entries {
		source, readErr := disk.Read(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			t.Fatal(readErr)
		}

		files[entry.Name()] = string(source)
	}

	return files
}

const scenarioRunOriginalState = `package steps
%s
type State struct {
 Name string ` + "`json:\"name\"`" + `
 // Count belongs to the definitions.
 Count int
 Values []string
 Flags map[string]bool
 Detail *Detail
}
func (state *State) Reset() { state.Count = 0 }
func (state State) Label() string { return state.Name }
type Detail struct { Ready bool }
const label = "kept"
var initial = Detail{Ready: true}
func describe() string { return label }
`

const scenarioRunEmbeddedState = `package steps
%s
type State struct {
 %s

 Name string ` + "`json:\"name\"`" + `
 // Count belongs to the definitions.
 Count int
 Values []string
 Flags map[string]bool
 Detail *Detail
}
func (state *State) Reset() { state.Count = 0 }
func (state State) Label() string { return state.Name }
type Detail struct { Ready bool }
const label = "kept"
var initial = Detail{Ready: true}
func describe() string { return label }
`
