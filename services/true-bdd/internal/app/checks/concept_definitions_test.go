package checks_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

const conceptProduct = `concepts:
  when: [{concept: halving, means: The Owner halves a whole number., terms: []}]
  then: [{concept: result, means: The calculator prints the half., terms: []}]
`

const conceptRegister = "package steps\n\nfunc Register(suite Registrar) {\n" +
	"\tsuite.Step(`^the Owner halves (-?\\d+)$`, halving)\n" +
	"\tsuite.Step(`^the result is (-?\\d+)$`, result)\n" +
	"\tsuite.Step(`^the result is (\\d+)$`, positive)\n" +
	"\tsuite.Step(`^the Owner doubles (\\d+)$`, doubling)\n" +
	"\tsuite.Step(`^the Owner squares (\\d+)$`, squaring)\n}\n"

const conceptDefinitions = `package steps

type State struct{}

type Registrar interface{ Step(string, func(*State, []string) error) }

//truebdd:concept when "halving"
func halving(*State, []string) error { return nil }

//truebdd:concept then "result"
func result(*State, []string) error { return nil }

//truebdd:concept then "result"
func positive(*State, []string) error { return nil }

//truebdd:concept then "halving"
func doubling(*State, []string) error { return nil }
`

// On a concept host every definition a Scenario's step binds records a
// declared concept of the step's block, and no step binds two; a step
// bound to nothing yet is a gap build steps writes.
func TestConceptStepDefinitionsCheck(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		steps []catalogue.Step
		pass  bool
		want  string
	}{
		{"recorded", []catalogue.Step{{Block: catalogue.BlockWhen, Text: "the Owner halves 6"},
			{Block: catalogue.BlockThen, Text: "the result is -3"}}, true, "records a declared concept of its block"},
		{"unbound yet", []catalogue.Step{{Block: catalogue.BlockWhen, Text: "the Owner triples 6"}}, true,
			"1 step unbound yet"},
		{"bound twice", []catalogue.Step{{Block: catalogue.BlockThen, Text: "the result is 3"}}, false,
			`then step "the result is 3" binds 2 step definitions`},
		{"another block's concept", []catalogue.Step{{Block: catalogue.BlockWhen, Text: "the Owner doubles 6"}},
			false, `records then concept "halving", which is not a declared when concept`},
		{"no record", []catalogue.Step{{Block: catalogue.BlockWhen, Text: "the Owner squares 6"}}, false,
			"records no concept"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			item, input := conceptSuite(t, testCase.steps)

			verdict, err := checks.ForScenarios()["step-definitions"].Run(item, input)
			if err != nil {
				t.Fatal(err)
			}

			context := strings.Join(verdict.Context, "\n")
			if verdict.Pass != testCase.pass || !strings.Contains(context, testCase.want) {
				t.Fatalf("unexpected verdict: %+v", verdict)
			}
		})
	}
}

func conceptSuite(t *testing.T, steps []catalogue.Step) (*template.ScenarioCheckData, checks.Input) {
	t.Helper()

	root := t.TempDir()

	files := map[string]string{
		"docs/product/product.yaml":      conceptProduct,
		"tests/calc/steps/register.go":   conceptRegister,
		"tests/calc/steps/concepts.go":   conceptDefinitions,
		"tests/calc/steps/unrecorded.go": "package steps\n\nfunc squaring(*State, []string) error { return nil }\n",
	}
	for path, body := range files {
		err := disk.Write(filepath.Join(root, path), []byte(body), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}

	item := &template.ScenarioCheckData{ID: "calc-001", Path: filepath.Join(root, "tests/calc/calc_test.go"),
		Phrases: steps}

	return item, checks.Input{Docs: map[string]string{docs.KeyProduct: filepath.Join(root, "docs/product/product.yaml")}}
}
