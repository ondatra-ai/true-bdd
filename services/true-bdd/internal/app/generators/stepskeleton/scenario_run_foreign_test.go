package stepskeleton_test

import (
	"fmt"
	"go/format"
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

func TestScenarioRunEmbeddingKeepsForeignImports(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct{ name, imports, alias string }{
		{"foreign alias", `import testkit "example.com/other"`, "testkit1"},
		{"occupied aliases", `import (
 testkit "example.com/other"
 testkit1 "example.com/another"
 "example.com/testkit2"
)`, "testkit3"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(t.TempDir(), "steps")
			source := fmt.Sprintf(scenarioRunOriginalState, testCase.imports)
			source = strings.Replace(source, "type State struct {", "type State struct {\n *testkit.ScenarioRun", 1)
			file := filepath.Join(dir, "definitions.go")

			err := disk.Write(file, []byte(source), disk.Shared)
			if err != nil {
				t.Fatal(err)
			}

			err = stepskeleton.EnsureScenarioRun(dir)
			if err != nil {
				t.Fatal(err)
			}

			got := scenarioRunFiles(t, dir)
			wantImports := scenarioRunImports(t, parseScenarioRun(t, source))
			wantImports[scenarioRunTestkitImport] = testCase.alias

			if imports := scenarioRunImports(t, parseScenarioRun(t, got["definitions.go"])); !maps.Equal(imports, wantImports) {
				t.Fatalf("imports: got %v, want %v", imports, wantImports)
			}

			want := strings.Replace(source, "type State struct {", "type State struct {\n *"+testCase.alias+".ScenarioRun", 1)
			header := fmt.Sprintf("package steps\nimport %s %q", testCase.alias, scenarioRunTestkitImport)
			want = strings.Replace(want, "package steps", header, 1)
			requireScenarioRunDeclarationsUnchanged(t, got["definitions.go"], want)

			_, present, err := stepskeleton.InspectScenarioRun(dir)
			if err != nil || !present {
				t.Fatalf("repaired field: present=%v, error=%v", present, err)
			}

			err = stepskeleton.EnsureScenarioRun(dir)
			if err != nil {
				t.Fatal(err)
			}

			if after := scenarioRunFiles(t, dir); !maps.Equal(after, got) {
				t.Fatal("repeated repair changed files")
			}
		})
	}
}

func requireScenarioRunDeclarationsUnchanged(t *testing.T, got, want string) {
	t.Helper()

	gotTree := parseScenarioRun(t, got)
	wantTree := parseScenarioRun(t, want)
	gotState, gotDeclarations := scenarioRunDeclarations(t, gotTree)
	wantState, wantDeclarations := scenarioRunDeclarations(t, wantTree)

	if fmt.Sprint(gotDeclarations) != fmt.Sprint(wantDeclarations) {
		t.Fatalf("declarations: got %v, want %v", gotDeclarations, wantDeclarations)
	}

	gotFields := renderScenarioRunNode(t, gotState)
	wantFields := renderScenarioRunNode(t, wantState)

	if gotFields != wantFields {
		t.Fatalf("State: got %s, want %s", gotFields, wantFields)
	}

	formatted, err := format.Source([]byte(got))
	if err != nil || string(formatted) != got {
		t.Fatalf("repair is not formatted: %v", err)
	}
}
