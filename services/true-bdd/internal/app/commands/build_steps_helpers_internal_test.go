package commands

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

type helperCritiqueReply struct {
	answer, system, user string
	model                provider.ModelRef
}

func (reply *helperCritiqueReply) ExecutePromptWithSystem(_ context.Context, _ provider.Role, system, user string,
	model provider.ModelRef, _ provider.ExecutionMode, _ string, _ ...provider.TurnOptions,
) (string, error) {
	reply.system, reply.user = system, user
	reply.model = model

	return reply.answer, nil
}

func helperCritiqueExecution(t *testing.T, reply *helperCritiqueReply) *buildStepsExecution {
	t.Helper()

	repository, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}

	t.Chdir(t.TempDir())

	models, err := provider.NewRegistry(map[string]string{repairTier: "claude:test", "quick": "claude:verify"},
		map[provider.Role]string{
			provider.RolePrompt: repairTier, provider.RoleFix: repairTier, provider.RoleApply: repairTier,
		})
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Write("architecture.yaml", []byte("architecture: {}\n"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	return &buildStepsExecution{architectureFile: "architecture.yaml", deps: BuildStepsDeps{
		TmpDir: t.TempDir(), Critic: validate.NewStepCritic(reply, models,
			filepath.Join(repository, "templates/build-steps.critique.system.prompt.tpl"),
			filepath.Join(repository, "templates/build-steps.critique.prompt.tpl"))}}
}

func TestVerificationUsesChecklistQuestionTierAndArtifacts(t *testing.T) {
	reply := &helperCritiqueReply{answer: `{"answer":"pass","findings":[]}`}
	execution := helperCritiqueExecution(t, reply)
	writeHelperEvidence(t, "tests/calc/steps/result.go", "package steps\nfunc observeAnswer() {}\n")

	item := &BuildStepsItem{Suite: repairSuite, subjectID: "answer",
		Concept: &BuildStepsConcept{Definitions: []stepskeleton.Definition{{Function: "observeAnswer"}}}}
	prompt := verificationTestPrompt()
	prompt.Prompt.Model = "quick"

	_, err := execution.answerVerification(t.Context(), item, prompt)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(reply.user, prompt.Prompt.Question) || reply.model.Model != "verify" {
		t.Fatalf("verification lost its checklist question or model: %s, %+v", reply.user, reply.model)
	}

	stem := filepath.Join(execution.deps.TmpDir, "02-answer-checklist-build-steps-step-verification")
	for _, suffix := range []string{"system.txt", "user.txt", "response.txt", "result.yaml"} {
		_, err = disk.Read(stem + "-" + suffix)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func writeHelperEvidence(t *testing.T, path, content string) {
	t.Helper()

	err := disk.Write(path, []byte(content), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}
}

func verificationTestPrompt() *checklist.PromptWithContext {
	return &checklist.PromptWithContext{Index: 2, SectionID: "build steps", CriterionID: stepVerificationID,
		Prompt: checklist.Prompt{Question: "Does the definition match its steps under its concept and binding?",
			Model: repairTier}}
}

func TestCritiqueRefusesProductWorkInAHelper(t *testing.T) {
	reply := &helperCritiqueReply{answer: `{"answer":"fail","findings":[{"definition":"observeAnswer",` +
		`"category":"computes_assertion",` +
		`"finding":"helpers/answer.go computes the product answer locally"}]}`}
	execution := helperCritiqueExecution(t, reply)
	writeHelperEvidence(t, "tests/calc/steps/result.go", "package steps\nfunc observeAnswer() { helpers.Answer(6) }\n")
	writeHelperEvidence(t, "tests/calc/helpers/answer.go",
		"package helpers\n// answer-contract.txt states the expected answer.\n"+
			"func Answer(n int) int { return n / 2 }\n")

	item := &BuildStepsItem{Suite: repairSuite,
		Concept: &BuildStepsConcept{Definitions: []stepskeleton.Definition{{Function: "observeAnswer"}}},
		Context: "concept: {concept: result}\nfixture_root: tests/calc/fixtures\nfixture_inputs: [{fixture: answer}]\n"}

	findings, err := execution.critique(t.Context(), item, verificationTestPrompt())
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"tests/calc/helpers/answer.go", "return n / 2", "tests/calc/steps/result.go"} {
		if !strings.Contains(reply.user, want) {
			t.Errorf("critique lacks %q", want)
		}
	}

	if !strings.Contains(reply.system, "helper") || !strings.Contains(reply.system, "computes_assertion") {
		t.Fatal("critique policy does not judge helper product work")
	}

	if len(findings) != 1 || findings[0].Category != "computes_assertion" {
		t.Fatalf("helper finding lost: %v", findings)
	}
}
