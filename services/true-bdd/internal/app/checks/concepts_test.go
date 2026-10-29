package checks_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

func TestConceptTerms(t *testing.T) {
	t.Parallel()

	for _, checkName := range []string{"declared-terms", "term-mappings"} {
		t.Run(checkName, func(t *testing.T) {
			t.Parallel()
			item, input := conceptInput(t, "", "")

			verdict, err := checks.ForScenarios()[checkName].Run(item, input)
			if err != nil {
				t.Fatal(err)
			}

			context := strings.Join(verdict.Context, "\n")
			if verdict.Pass || !strings.Contains(context, "when concept 'halve'") || !strings.Contains(context, "number") {
				t.Fatalf("missing concept and term finding: %+v", verdict)
			}
		})
	}
}

func TestConceptBindings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, bindings, want string
		pass                 bool
	}{
		{"declared", "[{concept: halve, block: when, service: calc, references: " +
			"[{element: {page: Calc, role: button, name: Halve}}], does: Call " +
			"halve.}]", "every concept binding", true},
		{"undeclared concept",
			"[{concept: missing, block: when, service: calc, intent: Call it.}]",
			"concept 'missing'", false},
		{"wrong block",
			"[{concept: halve, block: given, service: calc, intent: Call it.}]",
			"given concept 'halve'", false},
		{"duplicate", "[{concept: halve, block: when, service: calc, intent: Call it.}, " +
			"{concept: halve, block: when, service: calc, intent: Call it.}]", "has 2 Concept bindings", false},
		{"undeclared operation", "[{concept: halve, block: when, service: calc, references: " +
			"[{command: invented}], does: Call it.}]", "invented", false},
		{"unknown service",
			"[{concept: halve, block: when, service: missing, intent: Call it.}]",
			"service missing", false},
		{"UI intent", "[{concept: halve, block: when, service: calc, intent: Call it.}]", "every concept binding", true},
		{"intent and operation",
			"[{concept: halve, block: when, service: calc, intent: Call it., does: Call it.}]",
			"replace does", false},
		{"no binding", "", "has no Concept binding", false},
		{"empty operation", "[{concept: halve, block: when, service: calc}]", "does", false},
		{"missing references", "[{concept: halve, block: when, service: calc, does: Call it.}]", "references", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			item, input := conceptInput(t, testCase.bindings, "")

			verdict, err := checks.ForScenarios()["phrase-mappings"].Run(item, input)
			if err != nil {
				t.Fatal(err)
			}

			context := strings.Join(verdict.Context, "\n")
			if verdict.Pass != testCase.pass || !strings.Contains(context, testCase.want) {
				t.Fatalf("unexpected verdict: %+v", verdict)
			}

			if !testCase.pass && !strings.Contains(context, "concept '") {
				t.Fatalf("missing concept: %+v", verdict)
			}
		})
	}
}

func conceptInput(t *testing.T, bindings, extra string) (*template.ScenarioCheckData, checks.Input) {
	t.Helper()
	root := t.TempDir()
	product := "concepts:\n  when: [{concept: halve, means: Call the number " +
		"command., terms: [number]}]\n" + extra

	arch := `architecture:
  services:
  - name: calc
    public_interface:
      kind: ui
      elements: [{page: Calc, role: button, name: Halve}]
`
	if bindings != "" {
		arch += "  concepts: " + bindings + "\n"
	}

	input := checks.Input{Docs: map[string]string{
		docs.KeyProduct:          filepath.Join(root, "product.yaml"),
		docs.KeyArchitectureYAML: filepath.Join(root, "architecture.yaml"),
	}}
	for key, body := range map[string]string{docs.KeyProduct: product, docs.KeyArchitectureYAML: arch} {
		err := disk.Write(input.Docs[key], []byte(body), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}

	return &template.ScenarioCheckData{ID: "halve-001", Service: "calc"}, input
}
