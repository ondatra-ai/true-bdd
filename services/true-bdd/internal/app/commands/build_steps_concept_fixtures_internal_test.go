package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// A concept Item's context carries the fixture inputs of the Scenarios
// using its steps: every file of each fixture such a Scenario names by a
// quoted value, its recordings left out, and no other Scenario's fixture.
func TestConceptContextHoldsBoundScenariosFixtureInputs(t *testing.T) {
	t.Chdir(t.TempDir())

	for path, content := range map[string]string{
		"tests/calc/fixtures/offline/bin/go":                 "#!/bin/sh\nexec \"$CALC_REAL_GO\" \"$@\"\n",
		"tests/calc/fixtures/offline/README":                 "Offline toolchain.\n",
		"tests/calc/fixtures/offline/cassettes/claude-001/x": "a recording, not an input\n",
		"tests/calc/fixtures/other/marker":                   "another Scenario's fixture\n",
	} {
		err := os.MkdirAll(filepath.Dir(path), 0o750)
		if err != nil {
			t.Fatal(err)
		}

		err = os.WriteFile(path, []byte(content), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	scenarios := []*registry.RegistryScenario{
		fixtureScenario("calc-001", `the toolchain is the "offline" fixture's`, halveSix),
		fixtureScenario("calc-002", `the toolchain is the "other" fixture's`, "the Owner doubles 6"),
		fixtureScenario("calc-003", `the toolchain is the "missing" fixture's`, "the Owner halves 7"),
	}
	item := &BuildStepsItem{Suite: "tests/calc/steps", Service: repairService, Concept: &BuildStepsConcept{
		Block: catalogue.BlockWhen, Name: repairConcept, Steps: []string{halveSix, "the Owner halves 7"},
	}}

	err := describeConceptItem(item, &architecture.Architecture{}, conceptSpecification{},
		&conceptDiscovery{scenarios: scenarios})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"fixture_inputs:", "fixture: tests/calc/fixtures/offline", "- calc-001",
		"path: tests/calc/fixtures/offline/bin/go", `exec "$CALC_REAL_GO" "$@"`, "Offline toolchain."} {
		if !strings.Contains(item.Context, want) {
			t.Errorf("context lacks %q:\n%s", want, item.Context)
		}
	}

	for _, unwanted := range []string{"a recording", "another Scenario's fixture", "fixtures/missing"} {
		if strings.Contains(item.Context, unwanted) {
			t.Errorf("context holds %q:\n%s", unwanted, item.Context)
		}
	}
}

func fixtureScenario(id, given, when string) *registry.RegistryScenario {
	return &registry.RegistryScenario{ID: id, Description: id, Service: repairService, Path: "tests/calc/calc_test.go",
		Steps: template.MergedSteps{Given: []string{given}, When: []string{when}},
		Statements: []registry.Statement{
			{Block: catalogue.BlockGiven, Text: given}, {Block: catalogue.BlockWhen, Text: when},
		}}
}
