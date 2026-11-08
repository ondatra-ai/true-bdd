package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerificationCarriesNoFixtureFileOrScenarioCorpus(t *testing.T) {
	t.Chdir(t.TempDir())

	for path, content := range map[string]string{
		"tests/calc/fixtures/offline/prep.sh":                 "export CALC_REAL_GO=/real/go\n",
		"tests/calc/fixtures/offline/input/bin/go":            "exec $CALC_REAL_GO\n",
		"tests/calc/fixtures/offline/input/docs/product.yaml": "unrelated product corpus\n",
		"tests/calc/fixtures/offline/cassettes/claude-001/x":  "recorded answer\n",
	} {
		err := os.MkdirAll(filepath.Dir(path), 0750)
		if err != nil {
			t.Fatal(err)
		}

		err = os.WriteFile(path, []byte(content), 0600)
		if err != nil {
			t.Fatal(err)
		}
	}

	item := &BuildStepsItem{Context: "concept: {concept: toolchain}" +
		"\nregistry: docs/scenarios.yaml\nscenarios: [calc-001]" +
		"\nfixture_inputs:\n- fixture: tests/calc/fixtures/offline\n  scenarios: [calc-001]\n"}

	rendered, err := critiqueContext(item, `exec.Command("go")`)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(rendered, "concept: toolchain") {
		t.Fatal(rendered)
	}

	for _, unwanted := range []string{"CALC_REAL_GO", "input/bin/go", "unrelated product corpus",
		"recorded answer", "fixture_inputs", "calc-001", "scenarios.yaml"} {
		if strings.Contains(rendered, unwanted) {
			t.Errorf("verification includes %q: %s", unwanted, rendered)
		}
	}
}
