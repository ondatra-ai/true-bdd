package validate_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
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

func TestProposalRefusedByTheEngineCheckFailsWithItsFindings(t *testing.T) {
	evaluator, reply, _ := conceptRequestEvaluator(t)
	bindHalve(t)

	reply.response = `{"patterns":[{"pattern":"^halve (-?\\d+)$","captures":[{"name":"value","means":"operand"}],` +
		`"serves":"Press Halve once."}]}`
	subject := proposalSubject()

	result, err := evaluator.EvaluateOne(t.Context(), subject, subject.ID, proposalPrompt(), t.TempDir())
	if err != nil || result.Status != checklist.StatusFail || len(subject.Patterns) != 0 {
		t.Fatalf("colliding proposal: %+v, %v", result, err)
	}

	if len(result.Context) != 1 || !strings.Contains(result.Context[0], `also binds when step "halve 6"`) {
		t.Fatalf("collision finding: %q", result.Context)
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

	for _, answer := range []string{`{"patterns":[`, `{"patterns":[{"pattern":"^halve$"}]}`, `{"patterns":[]}`} {
		reply.response = answer
		subject := proposalSubject()

		_, err := evaluator.EvaluateOne(t.Context(), subject, subject.ID, proposalPrompt(), t.TempDir())
		if err == nil {
			t.Errorf("malformed proposal passed: %s", answer)
		}
	}
}
