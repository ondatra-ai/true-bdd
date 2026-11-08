package stepskeleton_test

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
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

			sibling := filepath.Join(dir, "other.go")

			err = disk.Write(sibling, []byte(scenarioRunSibling), disk.Shared)
			if err != nil {
				t.Fatal(err)
			}

			err = stepskeleton.EnsureScenarioRun(dir)
			if err != nil {
				t.Fatal(err)
			}

			got, err := disk.Read(file)
			if err != nil {
				t.Fatal(err)
			}

			siblingBytes, err := disk.Read(sibling)
			if err != nil {
				t.Fatal(err)
			}

			if string(siblingBytes) != scenarioRunSibling {
				t.Fatalf("embedding changed sibling bytes: got %q, want %q", siblingBytes, scenarioRunSibling)
			}

			original := parseScenarioRun(t, source)

			result := parseScenarioRun(t, string(got))
			if imports := scenarioRunImports(t, result); !maps.Equal(imports, testCase.wantImports) {
				t.Fatalf("imports: got %v, want %v", imports, testCase.wantImports)
			}

			state, declarations := scenarioRunDeclarations(t, result)

			_, wantDeclarations := scenarioRunDeclarations(t, original)
			if !slices.Equal(declarations, wantDeclarations) {
				t.Fatalf("definition-owned declarations: got %q, want %q", declarations, wantDeclarations)
			}

			requireScenarioRunFields(t, state, testCase.field)
		})
	}
}

func scenarioRunImportCases() []struct {
	name, imports, field string
	wantImports          map[string]string
} {
	return []struct {
		name, imports, field string
		wantImports          map[string]string
	}{
		{"grouped imports", "import (\n\"fmt\"\n\"strings\"\n)", plainScenarioRunField,
			map[string]string{"fmt": "", "strings": "", scenarioRunTestkitImport: ""}},
		{"single import", "import \"fmt\"", plainScenarioRunField,
			map[string]string{"fmt": "", scenarioRunTestkitImport: ""}},
		{"no imports", "", plainScenarioRunField,
			map[string]string{scenarioRunTestkitImport: ""}},
		{"aliased testkit", "import harness \"github.com/ondatra-ai/true-bdd/pkg/testkit\"", "*harness.ScenarioRun",
			map[string]string{scenarioRunTestkitImport: "harness"}},
		{"dot testkit", "import . \"github.com/ondatra-ai/true-bdd/pkg/testkit\"", "*ScenarioRun",
			map[string]string{scenarioRunTestkitImport: "."}},
	}
}

const scenarioRunTestkitImport = "github.com/ondatra-ai/true-bdd/pkg/testkit"

const plainScenarioRunField = "*testkit.ScenarioRun"

const scenarioRunSibling = "package steps\nfunc other( )string{return \"untouched\"}\n"

func TestScenarioRunRepeatedCallLeavesEveryFileUnchanged(t *testing.T) {
	t.Parallel()

	for _, testCase := range scenarioRunImportCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(t.TempDir(), "steps")
			for name, source := range map[string]string{
				"definitions.go": fmt.Sprintf(scenarioRunOriginalState, testCase.imports),
				"other.go":       scenarioRunSibling,
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

			want := map[string]string{"definitions.go": testCase.source, "other.go": scenarioRunSibling}
			for name, source := range want {
				err := disk.Write(filepath.Join(dir, name), []byte(source), disk.Shared)
				if err != nil {
					t.Fatal(err)
				}
			}

			err := stepskeleton.EnsureScenarioRun(dir)
			if err != nil {
				t.Fatal(err)
			}

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

func parseScenarioRun(t *testing.T, source string) *ast.File {
	t.Helper()

	tree, err := parser.ParseFile(token.NewFileSet(), "definitions.go", source, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	return tree
}

func renderScenarioRunNode(t *testing.T, node any) string {
	t.Helper()

	var rendered bytes.Buffer

	err := format.Node(&rendered, token.NewFileSet(), node)
	if err != nil {
		t.Fatal(err)
	}

	return rendered.String()
}

func scenarioRunImports(t *testing.T, tree *ast.File) map[string]string {
	t.Helper()

	imports := make(map[string]string)

	for _, imported := range tree.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			t.Fatal(err)
		}

		if _, exists := imports[path]; exists {
			t.Fatalf("duplicate import %q", path)
		}

		name := ""
		if imported.Name != nil {
			name = imported.Name.Name
		}

		imports[path] = name
	}

	return imports
}

func scenarioRunDeclarations(t *testing.T, tree *ast.File) (*ast.StructType, []string) {
	t.Helper()

	var state *ast.StructType

	var declarations []string

	for _, declaration := range tree.Decls {
		if general, ok := declaration.(*ast.GenDecl); ok {
			if general.Tok == token.IMPORT {
				continue
			}

			if typed, ok := general.Specs[0].(*ast.TypeSpec); ok && typed.Name.Name == "State" {
				if state != nil || len(general.Specs) != 1 {
					t.Fatal("expected one State declaration")
				}

				state, _ = typed.Type.(*ast.StructType)

				continue
			}
		}

		declarations = append(declarations, renderScenarioRunNode(t, declaration))
	}

	if state == nil {
		t.Fatal("State struct missing")
	}

	return state, declarations
}

func requireScenarioRunFields(t *testing.T, state *ast.StructType, embedded string) {
	t.Helper()

	var fields []string

	embeddedCount := 0

	for _, field := range state.Fields.List {
		fieldType := renderScenarioRunNode(t, field.Type)
		if len(field.Names) == 0 && fieldType == embedded {
			embeddedCount++

			continue
		}

		for _, name := range field.Names {
			if name.Name == "Count" && field.Doc.Text() != "Count belongs to the definitions.\n" {
				t.Fatalf("Count doc comment: got %q", field.Doc.Text())
			}
		}

		fields = append(fields, renderScenarioRunField(t, field))
	}

	if embeddedCount != 1 {
		t.Fatalf("embedded field %s: got %d, want 1", embedded, embeddedCount)
	}

	want := []string{
		"Name string `json:\"name\"`", "Count int", "Values []string", "Flags map[string]bool", "Detail *Detail",
	}
	if !slices.Equal(fields, want) {
		t.Fatalf("definition-owned fields: got %q, want %q", fields, want)
	}
}

func renderScenarioRunField(t *testing.T, field *ast.Field) string {
	t.Helper()

	declaration := &ast.StructType{Fields: &ast.FieldList{List: []*ast.Field{
		{Names: field.Names, Type: field.Type, Tag: field.Tag},
	}}}
	rendered := renderScenarioRunNode(t, declaration)

	return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(rendered, "struct {"), "}"))
}
