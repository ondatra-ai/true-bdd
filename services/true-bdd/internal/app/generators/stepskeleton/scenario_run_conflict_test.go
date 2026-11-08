package stepskeleton_test

import (
	"errors"
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

func TestScenarioRunConflictRefusesWithoutWrites(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct{ name, source string }{
		{"foreign testkit", `import testkit "example.com/other"
type State struct{ *testkit.ScenarioRun }`},
		{"other package", `import other "example.com/other"
type State struct{ other.ScenarioRun }`},
		{"local type", `type ScenarioRun struct{}
type State struct{ *ScenarioRun }`},
		{"generic local type", `type ScenarioRun[T any] struct{}
type State struct{ *ScenarioRun[int] }`},
		{"generic other package", `import other "example.com/other"
type State struct{ other.ScenarioRun[int, string] }`},
		{"named field", `type State struct{ ScenarioRun string }`},
		{"multiple names", `type State struct{ Other, ScenarioRun string }`},
		{"named harness field", `import %q
type State struct{ ScenarioRun *testkit.ScenarioRun }`},
		{"harness value", `import %q
type State struct{ testkit.ScenarioRun }`},
		{"harness and conflict", `import %q
type State struct{ *testkit.ScenarioRun; ScenarioRun string }`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(t.TempDir(), "steps")
			writeScenarioRunInspectionSource(t, dir, testCase.source)

			err := disk.Write(filepath.Join(dir, "other.go"), []byte(scenarioRunSibling), disk.Shared)
			if err != nil {
				t.Fatal(err)
			}

			before := scenarioRunFiles(t, dir)

			file, present, inspectErr := stepskeleton.InspectScenarioRun(dir)
			if file != filepath.Join(dir, "definitions.go") || present {
				t.Errorf("conflicting field inspection: file=%q, present=%v", file, present)
			}

			requireScenarioRunConflict(t, inspectErr, file)

			if !maps.Equal(before, scenarioRunFiles(t, dir)) {
				t.Error("inspection changed files")
			}

			requireScenarioRunConflict(t, stepskeleton.EnsureScenarioRun(dir), file)

			if !maps.Equal(before, scenarioRunFiles(t, dir)) {
				t.Error("refused repair changed files")
			}
		})
	}
}

func requireScenarioRunConflict(t *testing.T, err error, file string) {
	t.Helper()

	if !errors.Is(err, stepskeleton.ErrScenarioRunConflict) {
		t.Errorf("expected distinct ScenarioRun conflict: %v", err)
	}

	if err == nil {
		t.Error("expected ScenarioRun field conflict")

		return
	}

	for _, want := range []string{file, "State", "ScenarioRun", "conflict"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("conflict missing %q: %v", want, err)
		}
	}
}
