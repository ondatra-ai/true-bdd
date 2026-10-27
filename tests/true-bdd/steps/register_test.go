package steps_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"github.com/ondatra-ai/true-bdd/tests/true-bdd/steps"
)

// suiteService is the service the throwaway architecture declares and
// every throwaway scenario names.
const suiteService = "true-bdd"

// A number slot takes digits or `no` for zero (the catalogue's template
// grammar), so every step phrased with one binds when it says `no`.
func TestEveryNumberSlotTakesTheWordNo(t *testing.T) {
	t.Parallel()

	texts := []string{
		`the command exits with code no`,
		`the file "out.txt" has exactly no lines`,
		`exactly no files matching "tests/*.go" are created`,
		`the story "docs/product/stories/s.yaml" has no acceptance criteria`,
		`exactly no scenarios in "docs/scenarios.yaml" come from acceptance criterion "s-001-x"`,
		`the engine spawned exactly no test runners`,
		`the engine dispatched no AI turns`,
		`the engine dispatched no fix AI turns to "claude"`,
		`the shell command "true" run in "." exits with code no`,
	}

	unbound := unboundOf(t, "then", texts)
	if len(unbound) > 0 {
		t.Fatalf("number-slot steps saying `no` bind no definition: %q", unbound)
	}
}

// `the project's [browser ]test dependencies are installed` binds exactly
// what its template matches: both spellings, and nothing else.
func TestTheDependenciesStepBindsOnlyItsTemplate(t *testing.T) {
	t.Parallel()

	inside := []string{
		`the project's test dependencies are installed`,
		`the project's browser test dependencies are installed`,
	}
	outside := []string{
		`the project's npm dependencies are installed`,
		`the project's browser dependencies are installed`,
	}

	unbound := unboundOf(t, "given", append(slices.Clone(inside), outside...))

	for _, text := range inside {
		if slices.Contains(unbound, text) {
			t.Errorf("%q is the template's own wording, but binds no definition", text)
		}
	}

	for _, text := range outside {
		if !slices.Contains(unbound, text) {
			t.Errorf("%q is outside the template, but a definition binds it", text)
		}
	}
}

// unboundOf registers this suite's definitions over a throwaway registry
// holding each text as a one-step scenario of the block (`given`, `when` or
// `then`, as the registry spells it), and returns the texts no definition binds.
func unboundOf(t *testing.T, block string, texts []string) []string {
	t.Helper()

	dir := t.TempDir()
	architecture := filepath.Join(dir, "architecture.yaml")
	registry := filepath.Join(dir, "scenarios.yaml")

	writeYAML(t, architecture, map[string]any{"architecture": map[string]any{
		"testing":  map[string]any{"framework": "go-test"},
		"services": []map[string]any{{"name": suiteService}},
	}})

	scenarios := map[string]any{}
	for index, text := range texts {
		scenarios[fmt.Sprintf("s-%03d", index+1)] = map[string]any{
			"description":  text,
			"service":      suiteService,
			"merged_steps": map[string]any{block: []string{text}},
		}
	}

	writeYAML(t, registry, map[string]any{"scenarios": scenarios})

	suite, err := bddgo.New[steps.State](bddgo.Options{
		Registry: registry, Architecture: architecture, Suite: suiteService,
	})
	if err != nil {
		t.Fatalf("build the suite: %v", err)
	}

	owned, err := suite.Owned()
	if err != nil {
		t.Fatalf("read the registry: %v", err)
	}

	if len(owned) != len(texts) {
		t.Fatalf("the suite owns %d scenarios, want %d", len(owned), len(texts))
	}

	for _, scenario := range owned {
		if len(scenario.Steps) != 1 {
			t.Fatalf("scenario %s holds %d steps, want its one %s step", scenario.ID, len(scenario.Steps), block)
		}
	}

	steps.Register(suite)

	gaps, err := suite.Unbound()
	if err != nil {
		t.Fatalf("resolve the steps: %v", err)
	}

	unbound := make([]string, 0, len(gaps))
	for _, gap := range gaps {
		for _, step := range gap {
			unbound = append(unbound, step.Text)
		}
	}

	return unbound
}

// writeYAML writes a document the suite reads.
func writeYAML(t *testing.T, path string, document any) {
	t.Helper()

	data, err := yaml.Marshal(document)
	if err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}

	err = os.WriteFile(path, data, 0o600)
	if err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
