package stepskeleton_test

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

func TestScenarioRunInspectionNeverWrites(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name, source string
		present      bool
	}{
		{"plain", "import %q\ntype State struct{ *testkit.ScenarioRun }", true},
		{"alias", "import harness %q\ntype State struct{ *harness.ScenarioRun }", true},
		{"dot", "import . %q\ntype State struct{ *ScenarioRun }", true},
		{"missing field", "type State struct{ Name string }", false},
		{"named field", "import %q\ntype State struct{ Run *testkit.ScenarioRun }", false},
		{"missing State", "const name = 1", false},
		{"missing directory", "", false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			dir := filepath.Join(root, "steps")

			writeScenarioRunInspectionSource(t, dir, testCase.source)

			before := map[string]string{}
			if testCase.source != "" {
				before = scenarioRunFiles(t, dir)
			}

			file, present, err := stepskeleton.InspectScenarioRun(dir)
			if err != nil || present != testCase.present {
				t.Fatalf("present=%v, error=%v", present, err)
			}

			if file == "" {
				t.Fatal("missing State file location")
			}

			if testCase.source != "" {
				if after := scenarioRunFiles(t, dir); !maps.Equal(before, after) {
					t.Fatalf("inspection changed files: %v", after)
				}
			} else {
				_, err := os.Stat(dir)
				if !os.IsNotExist(err) {
					t.Fatalf("inspection created directory: %v", err)
				}
			}
		})
	}
}

func writeScenarioRunInspectionSource(t *testing.T, dir, source string) {
	t.Helper()

	if source == "" {
		return
	}

	if strings.Contains(source, "%q") {
		source = fmt.Sprintf(source, scenarioRunTestkitImport)
	}

	err := disk.Write(filepath.Join(dir, "definitions.go"), []byte("package steps\n"+source+"\n"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}
}
