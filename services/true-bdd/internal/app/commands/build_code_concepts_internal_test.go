package commands

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
)

const (
	doublingConcept = "doubling"
	doubleCommand   = "command: double"
	numberTerm      = "number"
)

const conceptCodeArchitecture = `architecture:
  services:
  - {name: calc, path: services/calc, language: go}
  - {name: other, path: services/other, language: go}
  concepts:
  - concept: ready
    block: given
    service: calc
    references: [{command: double}]
    does: Check that the double command is declared.
  - concept: doubling
    block: when
    service: calc
    references: [{command: double}]
    does: Run double with the step's number and keep its stdout.
  - concept: doubling
    block: when
    service: other
    intent: Never shown for a calc Scenario.
  - concept: result
    block: then
    service: calc
    references: [{command: double}]
    does: Compare the kept stdout with the step's number.
  terms:
  - {term: number, technical: The integer argument of double., observed_by: The step reads stdout.}
  - {term: unused, technical: Never linked., observed_by: Nothing.}
`

const conceptCodeProduct = `concepts:
  given:
  - {concept: ready, means: The calculator is ready to use., terms: []}
  when:
  - {concept: doubling, means: The Owner doubles a whole number., terms: [number]}
  - {concept: halving, means: The Owner halves a whole number., terms: [unused]}
  then:
  - {concept: result, means: The calculator prints the whole number., terms: [number]}
glossary:
- {term: number, means: A whole number.}
- {term: unused, means: Never linked.}
`

// The suite records a concept on every definition but the Then's second,
// so "the result is 7" links to nothing and only its step is left out.
const conceptCodeDefinitions = "package steps\n\n" +
	"type State struct{}\n\n" +
	"type Registrar interface{ Step(string, func(*State, []string) error) }\n\n" +
	"func Register(suite Registrar) {\n" +
	"\tsuite.Step(`^the calculator is ready$`, ready)\n" +
	"\tsuite.Step(`^the Owner doubles (-?\\d+)$`, doubling)\n" +
	"\tsuite.Step(`^the Owner halves (-?\\d+)$`, halving)\n" +
	"\tsuite.Step(`^the result is 6$`, result)\n" +
	"\tsuite.Step(`^the result is 7$`, unrecorded)\n}\n\n" +
	"//truebdd:concept given \"ready\"\nfunc ready(*State, []string) error { return nil }\n\n" +
	"//truebdd:concept when \"doubling\"\nfunc doubling(*State, []string) error { return nil }\n\n" +
	"//truebdd:concept when \"halving\"\nfunc halving(*State, []string) error { return nil }\n\n" +
	"//truebdd:concept then \"result\"\nfunc result(*State, []string) error { return nil }\n\n" +
	"func unrecorded(*State, []string) error { return nil }\n"

// A concept host's matched failure holds each concept its steps' definitions
// record, its terms' glossary entries and Word mappings, and its binding for
// the Scenario's service; a judge: clause and an unrecorded definition link none.
func TestCodeFailureEmbedsLinkedConcepts(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	for name, content := range map[string]string{
		"go.mod": "module host\n",
		"true-bdd/true-bdd.yaml": "documents:\n  product: docs/product/product.yaml\n" +
			"  scenarios_yaml: docs/scenarios.yaml\n",
		"docs/product/product.yaml": conceptCodeProduct,
		repairArchitecture:          conceptCodeArchitecture,
		conceptRegistry: `scenarios:
  calc-001-double:
    service: calc
    path: tests/calc/calc_test.go
    merged_steps:
      given: [the calculator is ready]
      when: [the Owner doubles 3]
      then: [the result is 6, the result is 7, "judge: the result reads well"]
`,
		"tests/calc/steps/steps.go": conceptCodeDefinitions,
	} {
		err := disk.Write(filepath.Join(root, name), []byte(content), disk.Private)
		if err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	failure := &testrunner.FailingTest{TestName: "host/tests/calc::TestCalc001Double",
		Framework: testrunner.FrameworkGoTest}
	arch := &architecture.Architecture{Services: []architecture.Service{{Name: repairService}, {Name: "other"}}}
	specifyCodeFailures(BuildCodeDeps{DocResolver: docs.NewResolver(cfg)}, arch,
		repairArchitecture, []*testrunner.FailingTest{failure})

	calc := func(references []string, does string) []catalogue.ConceptUse {
		return []catalogue.ConceptUse{{Service: repairService, References: references, Does: does}}
	}

	want := []catalogue.ConceptSpec{
		{Concept: "ready", Block: catalogue.BlockGiven, Means: "The calculator is ready to use.", Terms: []string{},
			Bindings: calc([]string{doubleCommand}, "Check that the double command is declared.")},
		{Concept: doublingConcept, Block: catalogue.BlockWhen, Means: "The Owner doubles a whole number.",
			Terms:    []string{numberTerm},
			Bindings: calc([]string{doubleCommand}, "Run double with the step's number and keep its stdout.")},
		{Concept: "result", Block: catalogue.BlockThen, Means: "The calculator prints the whole number.",
			Terms:    []string{numberTerm},
			Bindings: calc([]string{doubleCommand}, "Compare the kept stdout with the step's number.")},
	}
	if !reflect.DeepEqual(failure.Spec.Concepts, want) {
		t.Fatalf("linked concepts:\n got %+v\nwant %+v", failure.Spec.Concepts, want)
	}

	words := []catalogue.TermSpec{{Term: numberTerm, Means: []string{"A whole number."}, Mappings: []catalogue.Mapping{
		{Term: numberTerm, Technical: "The integer argument of double.", ObservedBy: "The step reads stdout."}}}}
	if !reflect.DeepEqual(failure.Spec.Terms, words) {
		t.Fatalf("linked words:\n got %+v\nwant %+v", failure.Spec.Terms, words)
	}
}

// The failing set's Specification lists each linked concept once, joining
// the bindings of every member's service, and each word once.
func TestFailingSetUnionsLinkedConcepts(t *testing.T) {
	doubling := func(service string) catalogue.ConceptSpec {
		return catalogue.ConceptSpec{Concept: doublingConcept, Block: catalogue.BlockWhen, Means: "Doubles.",
			Terms: []string{numberTerm}, Bindings: []catalogue.ConceptUse{{Service: service, Does: "Run " + service + "."}}}
	}
	result := catalogue.ConceptSpec{Concept: "result", Block: catalogue.BlockThen, Means: "Prints.", Terms: []string{}}
	number := catalogue.TermSpec{Term: numberTerm, Means: []string{"A whole number."}}

	item := &BuildCodeItem{Tests: []*testrunner.FailingTest{
		{Spec: catalogue.Specification{Concepts: []catalogue.ConceptSpec{doubling(repairService)},
			Terms: []catalogue.TermSpec{number}}},
		{LastRunPassed: true, Spec: catalogue.Specification{Concepts: []catalogue.ConceptSpec{result}}},
		{Spec: catalogue.Specification{Concepts: []catalogue.ConceptSpec{doubling("other"), doubling(repairService)},
			Terms: []catalogue.TermSpec{number}}},
	}}

	want := catalogue.ConceptSpec{Concept: doublingConcept, Block: catalogue.BlockWhen, Means: "Doubles.",
		Terms: []string{numberTerm}, Bindings: []catalogue.ConceptUse{{Service: repairService, Does: "Run calc."},
			{Service: "other", Does: "Run other."}}}

	spec := item.Specification()
	if !reflect.DeepEqual(spec.Concepts, []catalogue.ConceptSpec{want}) ||
		!reflect.DeepEqual(spec.Terms, []catalogue.TermSpec{number}) {
		t.Fatalf("union: %+v", spec)
	}
}
