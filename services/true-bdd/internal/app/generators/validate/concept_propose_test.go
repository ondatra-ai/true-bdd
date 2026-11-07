package validate_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

const (
	proposalItem    = "when:halve"
	proposalConcept = "halve"
)

func proposalSubject() *template.ConceptProposalData {
	return &template.ConceptProposalData{ID: proposalItem, Suite: "tests/calc", Service: linkTestService,
		Block: catalogue.BlockWhen, Concept: proposalConcept, Steps: []string{"halve -7"},
		Existing: []string{`^halve (\d+)$`},
		Suites: []catalogue.SuiteStep{
			{Block: catalogue.BlockWhen, Text: "halve -7", Item: proposalItem},
			{Block: catalogue.BlockWhen, Text: "halve 6", Bound: `^halve (\d+)$`},
		}}
}

func proposalPrompt() checklist.PromptWithContext {
	return checklist.PromptWithContext{Index: 2, Prompt: checklist.Prompt{Question: "Propose",
		With: map[string]any{conceptsParameter: "propose"}}}
}

func bindHalve(t *testing.T) {
	t.Helper()

	err := disk.Write("private-architecture.yaml", []byte("architecture:\n  services:\n  - name: calc\n"+
		"    public_interface:\n      kind: ui\n      elements: [{page: Calc, role: button, name: Halve}]\n"+
		"  concepts:\n  - concept: halve\n    block: when\n    service: calc\n"+
		"    references: [{element: {page: Calc, role: button, name: Halve}}]\n    does: Press Halve once.\n"),
		disk.Shared)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProposalLeavesPatternJudgementToTheChecklist(t *testing.T) {
	evaluator, reply, _ := conceptRequestEvaluator(t)
	bindHalve(t)

	reply.response = `{"patterns":[{"pattern":"^halve (-?\\d+)$","captures":[{"name":"value","means":"operand"}],` +
		`"serves":"Press Halve once."}]}`
	subject := proposalSubject()

	result, err := evaluator.EvaluateOne(t.Context(), subject, subject.ID, proposalPrompt(), t.TempDir())
	if err != nil || result.Status != checklist.StatusPass || len(subject.Patterns) != 1 {
		t.Fatalf("colliding proposal: %+v, %v", result, err)
	}

	for _, want := range []string{`"halve -7"`, `^halve (\\d+)$`, "Press Halve once.", "Halve"} {
		if !strings.Contains(reply.user, want) {
			t.Fatalf("proposal request lacks %s: %s", want, reply.user)
		}
	}

	if !strings.Contains(reply.schema, `"patterns"`) || !strings.Contains(reply.schema, `"captures"`) {
		t.Fatalf("proposal schema: %s", reply.schema)
	}
}

func TestProposalResentWithFindingsIsAcceptedOnceTheCheckPasses(t *testing.T) {
	evaluator, reply, _ := conceptRequestEvaluator(t)
	bindHalve(t)

	subject := proposalSubject()
	subject.Previous = []catalogue.ProposedPattern{{Pattern: `^halve (-?\d+)$`}}
	subject.Findings = []string{`pattern ^halve (-?\d+)$ also binds when step "halve 6"`}
	reply.response = `{"patterns":[{"pattern":"^halve (-\\d+)$","captures":[{"name":"value","means":"operand"}],` +
		`"serves":"Press Halve once."}]}`

	result, err := evaluator.EvaluateOne(t.Context(), subject, subject.ID, proposalPrompt(),
		filepath.Join(t.TempDir(), "tmp"))
	if err != nil || result.Status != checklist.StatusPass || len(subject.Patterns) != 1 ||
		subject.Patterns[0].Pattern != `^halve (-\d+)$` {
		t.Fatalf("accepted proposal: %+v %+v, %v", result, subject.Patterns, err)
	}

	if !strings.Contains(reply.user, `also binds when step \"halve 6\"`) {
		t.Fatalf("resend lacks the engine's findings: %s", reply.user)
	}
}

func TestMalformedProposalAnswerCrashes(t *testing.T) {
	evaluator, reply, _ := conceptRequestEvaluator(t)
	bindHalve(t)

	for _, answer := range []string{
		`{"patterns":[`, `{"patterns":[{"pattern":"^halve$"}]}`, `{"patterns":[]}`,
		`{"patterns":[{"pattern":"^halve (\\d+)$","captures":[{"name":"","means":"The number."}],"serves":"Run halve."}]}`,
		`{"patterns":[{"pattern":"^halve (\\d+)$","captures":[{"name":"number","means":""}],"serves":"Run halve."}]}`,
		`{"patterns":[{"pattern":"^halve (\\d+)$","captures":[{"name":"number","means":" \t\n"}],"serves":"Run halve."}]}`,
	} {
		reply.response = answer
		subject := proposalSubject()

		_, err := evaluator.EvaluateOne(t.Context(), subject, subject.ID, proposalPrompt(), t.TempDir())
		if err == nil {
			t.Errorf("malformed proposal passed: %s", answer)
		}
	}
}

func TestPatternBindingsFixCanRefuseWithoutChangingItsQuestion(t *testing.T) {
	evaluator, reply, _ := conceptRequestEvaluator(t)
	bindHalve(t)

	subject := proposalSubject()
	subject.Findings = []string{"no pattern can serve the closed binding"}
	reply.response = `{"patterns":[],"cannot_fix":{"reason":"The binding omits signed operands.",` +
 `"correction":"Decide whether negative operands are supported."}}`

	_, err := evaluator.EvaluateOne(t.Context(), subject, subject.ID, proposalPrompt(), t.TempDir())
	if !errors.Is(err, pkgerrors.ErrCannotFix) || !strings.Contains(err.Error(), "Decide whether negative operands") {
		t.Fatalf("pattern-bindings refusal: %v", err)
	}

	if len(subject.Patterns) != 0 || !strings.Contains(reply.schema, `"cannot_fix"`) {
		t.Fatalf("refusal produced patterns or lacks schema outcome: %+v %s", subject.Patterns, reply.schema)
	}

	if strings.Contains(reply.user, "This is the F of step-verification") {
		t.Fatalf("pattern-bindings received verification-only instructions: %s", reply.user)
	}
}
