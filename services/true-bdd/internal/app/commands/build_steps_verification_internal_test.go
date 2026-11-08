package commands

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
)

const verificationSectionPath = "build steps/step-verification"

func TestVerificationFixPlansPatternAndNewCapturesBeforeApplying(t *testing.T) {
	reply := &proposalReplies{answer: `{"patterns":[{"pattern":"^the Owner halves (-?\\d+)$",` +
		`"captures":[{"name":"number","means":"The complete number."}],"serves":"Run halve."}]}`}
	execution := repairExecution(t, reply)

	wrong, err := stepskeleton.WriteConcept(repairSuite, "when", repairConcept, wrongCapture)
	if err != nil {
		t.Fatal(err)
	}

	item := &BuildStepsItem{Suite: repairSuite, Service: repairService, subjectID: "when-halving",
		Concept: &BuildStepsConcept{Block: catalogue.BlockWhen, Name: repairConcept,
			Steps: []string{halveSix, "the Owner halves -7"}, Existing: []string{wrongCapture},
			Definitions: []stepskeleton.Definition{wrong}}}
	execution.verification = map[*BuildStepsItem][]validate.CritiqueFinding{item: {{
		Definition: wrong.Function, Category: "pattern_captures",
		Finding: "captures only the first digit",
	}}}
	failed := checklist.ValidationResult{SectionPath: verificationSectionPath}

	projected, err := execution.fixSubject(t.Context(), item, failed)
	if err != nil {
		t.Fatal(err)
	}

	if len(reply.requests) != 1 || !strings.Contains(reply.requests[0], "captures only the first digit") {
		t.Fatal(reply.requests)
	}

	if item.Concept.Definitions[0] != wrong {
		t.Fatal("proposal changed the skeleton")
	}

	if !strings.Contains(projected.Context, `pattern: ^the Owner halves (-?\d+)$`) {
		t.Fatal(projected.Context)
	}
	// A clarification or refinement reuses the proposed pattern, without another turn.
	_, err = execution.fixSubject(t.Context(), item, failed)
	if err != nil || len(reply.requests) != 1 {
		t.Fatalf("refinement reproposed: %v %v", err, reply.requests)
	}

	err = execution.beforeApply(t.Context(), item, failed)
	if err != nil {
		t.Fatal(err)
	}

	requireRewrittenSkeleton(t, item, wrong)
}

func TestVerificationBodyFixKeepsItsPatternAndOnlyReportedDefinitions(t *testing.T) {
	reply := &proposalReplies{}
	execution := repairExecution(t, reply)

	definition, err := stepskeleton.WriteConcept(repairSuite, "when", repairConcept, `^the Owner halves (-?\d+)$`)
	if err != nil {
		t.Fatal(err)
	}

	other, err := stepskeleton.WriteConcept(repairSuite, "when", repairConcept, `^halve another (-?\d+)$`)
	if err != nil {
		t.Fatal(err)
	}

	item := &BuildStepsItem{Suite: repairSuite, Service: repairService,
		Concept: &BuildStepsConcept{Block: catalogue.BlockWhen, Name: repairConcept,
			Definitions: []stepskeleton.Definition{definition, other}}}
	execution.verification = map[*BuildStepsItem][]validate.CritiqueFinding{item: {{
		Definition: definition.Function, Category: "concept_binding",
		Finding: "uses an undeclared licence gate",
	}}}

	projected, err := execution.fixSubject(t.Context(), item,
		checklist.ValidationResult{SectionPath: verificationSectionPath})
	if err != nil {
		t.Fatal(err)
	}

	if len(reply.requests) != 0 || len(projected.Concept.Definitions) != 1 ||
		projected.Concept.Definitions[0] != definition || len(item.Concept.Definitions) != 2 {
		t.Fatalf("body fix escaped findings: %+v %v", projected, reply.requests)
	}

	requireCoverageBodyScope(t, execution, item, other)
}

func requireCoverageBodyScope(t *testing.T, execution *buildStepsExecution, item *BuildStepsItem,
	other stepskeleton.Definition,
) {
	t.Helper()

	err := execution.beforeApply(t.Context(), item, checklist.ValidationResult{SectionPath: verificationSectionPath})
	if err != nil {
		t.Fatal(err)
	}

	writeHelperEvidence(t, other.File, "package steps\n//truebdd:concept when \"halving\"\nfunc "+other.Function+
		"(state *State,args []string) error { return nil }\n")

	if execution.preserveOtherBodies(item) == nil {
		t.Fatal("verification F allowed an unreported body change")
	}

	err = execution.beforeApply(t.Context(), item, checklist.ValidationResult{SectionPath: "build steps/step-coverage"})
	if err != nil {
		t.Fatal(err)
	}

	err = execution.preserveOtherBodies(item)
	if err != nil {
		t.Fatalf("coverage F retained a previous Q body restriction: %v", err)
	}
}
