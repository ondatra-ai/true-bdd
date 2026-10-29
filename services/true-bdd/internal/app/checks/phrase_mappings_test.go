package checks_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

func TestPhraseMappingCheck(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, mappings, want string
		pass                 bool
	}{
		{"missing", "", "has no Phrase mapping entry", false},
		{"other service", "[{template: halve, service: other, intent: Halve it}]", "has no Phrase mapping entry", false},
		{"duplicate", "[{template: halve, service: calc}, {template: halve, service: calc}]",
			"has 2 Phrase mapping entries", false},
		{"unknown reference", `[{template: halve, service: calc, does: Call halve.,
 references: [{element: {page: Calc, role: button, name: Missing}}]}]`, "Missing", false},
		{"mapped", `[{template: halve, service: calc, does: Call halve.,
 references: [{element: {page: Calc, role: button, name: Halve}}]}]`, "every matched phrase", true},
		{"intent UI", "[{template: halve, service: calc, intent: Halve whatever number is active.}]",
			"every matched phrase", true},
		{"missing explanation", `[{template: halve, service: calc,
 references: [{element: {page: Calc, role: button, name: Halve}}]}]`, "Halve", false},
		{"intent and operation", `[{template: halve, service: calc, intent: Halve it., does: Call halve.,
 references: [{element: {page: Calc, role: button, name: Halve}}]}]`, "intent", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			item, input := phraseCheckInput(t, testCase.mappings)
			verdict := runPhraseCheck(t, item, input)

			context := strings.Join(verdict.Context, "\n")
			if verdict.Pass != testCase.pass || !strings.Contains(context, testCase.want) {
				t.Fatalf("unexpected verdict: %+v", verdict)
			}

			if !testCase.pass && !strings.Contains(context, "halve") {
				t.Fatalf("phrase missing: %s", context)
			}

			item.UserStories = nil

			verdict = runPhraseCheck(t, item, input)
			if !verdict.Pass || !strings.Contains(strings.Join(verdict.Context, "\n"), "not checked") {
				t.Fatalf("unclaimed: %+v", verdict)
			}
		})
	}
}

func TestPhraseMappingsWithoutCataloguePassUnchecked(t *testing.T) {
	t.Parallel()
	item, input := phraseCheckInput(t, "")

	err := disk.Write(input.Docs["product"], []byte("title: Calculator\n"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	verdict := runPhraseCheck(t, item, input)
	if !verdict.Pass || !strings.Contains(strings.Join(verdict.Context, "\n"), "no catalogue") {
		t.Fatalf("no Catalogue: %+v", verdict)
	}
}

func phraseCheckInput(t *testing.T, mappings string) (*template.ScenarioCheckData, checks.Input) {
	t.Helper()
	root := t.TempDir()
	product := "catalogue: [{template: halve, block: when, means: Halve the number, terms: []}]\n"

	arch := `architecture:
  services:
  - name: calc
    public_interface:
      kind: ui
      elements: [{page: Calc, role: button, name: Halve}]
`
	if mappings != "" {
		arch += "  phrases: " + mappings + "\n"
	}

	for name, body := range map[string]string{"product.yaml": product, "architecture.yaml": arch} {
		err := disk.Write(filepath.Join(root, name), []byte(body), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}

	item := &template.ScenarioCheckData{
		ID: "halve-001", Service: "calc", UserStories: []template.StoryRef{{Story: "halve"}},
		Phrases: []catalogue.Step{{Block: "when", Text: "halve"}},
	}
	input := checks.Input{Docs: map[string]string{
		"product": filepath.Join(root, "product.yaml"), "architecture_yaml": filepath.Join(root, "architecture.yaml"),
	}}

	return item, input
}

func runPhraseCheck(t *testing.T, item *template.ScenarioCheckData, input checks.Input) checks.Verdict {
	t.Helper()

	check, exists := checks.ForScenarios()["phrase-mappings"]
	if !exists {
		t.Fatal("missing phrase-mappings check")
	}

	verdict, err := check.Run(item, input)
	if err != nil {
		t.Fatal(err)
	}

	return verdict
}
