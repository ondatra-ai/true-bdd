package validate_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
)

const buildCodeTemplates = "../../../../../../templates/"

const conceptHostSection = "\n\n## What the spec says these steps mean\n\n### Concepts\n\n" +
	"- `doubling` (when concept): The Owner doubles a whole number.\n" +
	"  Words: number\n" +
	"  Bound for calc through command: double: Run double with the step's number and keep its stdout.\n" +
	"  Bound for web by intent: The Owner presses Double.\n\n" +
	"- `ready` (given concept): The calculator is ready to use.\n" +
	"  Words: none\n" +
	"  The architecture binds it for no service of these steps.\n\n\n### Words\n\n#### number\n" +
	"- Means: An integer operand.\n\n" +
	"- In the implementation: The integer argument of double.\n- Observed by: The step reads stdout.\n\n\n\n\n\n"

func renderBuildCodeTurns(t *testing.T, spec catalogue.Specification) []string {
	t.Helper()

	item := &commands.BuildCodeItem{Tests: []*testrunner.FailingTest{{ID: "/go-test/go-test:calc::TestCalc", Spec: spec}}}

	plan, err := template.NewTemplateLoader[validate.FixPromptData](buildCodeTemplates +
		"build-code.fix-generator.prompt.tpl").LoadTemplate(validate.FixPromptData{Subject: item})
	if err != nil {
		t.Fatal(err)
	}

	writer, err := template.NewTemplateLoader[validate.FixApplierData](buildCodeTemplates +
		"build-code.fix-applier.prompt.tpl").LoadTemplate(validate.FixApplierData{Subject: item})
	if err != nil {
		t.Fatal(err)
	}

	return []string{plan, writer}
}

func specSection(t *testing.T, request string) string {
	t.Helper()

	start := strings.Index(request, "\n\n## What the spec says")
	end := strings.Index(request, "## Failed Prompt")

	if alternative := strings.Index(request, "## Proposed repair plan"); alternative >= 0 {
		end = alternative
	}

	if start < 0 || end < start {
		t.Fatalf("no Specification section in:\n%s", request)
	}

	return request[start:end]
}

const numberTerm = "number"

func numberWord() catalogue.TermSpec {
	return catalogue.TermSpec{Term: numberTerm, Means: []string{"An integer operand."},
		Mappings: []catalogue.Mapping{{Term: numberTerm, Technical: "The integer argument of double.",
			ObservedBy: "The step reads stdout."}}}
}

// Both repair turns of a concept host render each linked concept's meaning,
// words and bindings, then the same words; a host declaring no concepts gets
// no Specification section.
func TestBuildCodeTurnsEmbedTheLinkedConcepts(t *testing.T) {
	number := numberWord()

	conceptHost := catalogue.Specification{Concepts: []catalogue.ConceptSpec{
		{Concept: "doubling", Block: catalogue.BlockWhen, Means: "The Owner doubles a whole number.",
			Terms: []string{numberTerm}, Bindings: []catalogue.ConceptUse{
				{Service: "calc", References: []string{"command: double"},
					Does: "Run double with the step's number and keep its stdout."},
				{Service: "web", Intent: "The Owner presses Double."}}},
		{Concept: "ready", Block: catalogue.BlockGiven, Means: "The calculator is ready to use.", Terms: []string{}},
	}, Terms: []catalogue.TermSpec{number}}

	for _, request := range renderBuildCodeTurns(t, conceptHost) {
		if got := specSection(t, request); got != conceptHostSection {
			t.Errorf("concept host section:\n got %q\nwant %q", got, conceptHostSection)
		}
	}

	for _, request := range renderBuildCodeTurns(t, catalogue.Specification{}) {
		if strings.Contains(request, "What the spec says") {
			t.Errorf("a host declaring no concepts got a Specification section:\n%s", request)
		}
	}
}
