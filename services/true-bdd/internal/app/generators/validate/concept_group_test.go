package validate_test

import (
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
	"testing"
)

const linkTestService = "calc"
const groupTestSuite = "suite"
const linkTestStepText = "square six"

func TestGroupingUsesLinkThresholdAndAmbiguity(t *testing.T) {
	evaluator, reply, _ := conceptRequestEvaluator(t)
	reply.response = `{"assignments":[{"index":1,"concept":"square","score":9,"runner_up":"halve","runner_up_score":8}]}`

	for _, testCase := range []struct {
		threshold int
		status    checklist.Status
	}{{9, checklist.StatusPass}, {8, checklist.StatusFail}, {10, checklist.StatusFail}} {
		item := &template.ScenarioCheckData{ID: groupTestSuite, Service: linkTestService,
			Phrases: []catalogue.Step{{Block: catalogue.BlockWhen, Text: linkTestStepText}}}

		result, err := evaluator.EvaluateOne(t.Context(), item, item.ID, groupTestPrompt(testCase.threshold), t.TempDir())
		if err != nil || result.Status != testCase.status {
			t.Fatalf("threshold %d: %+v, %v", testCase.threshold, result, err)
		}
	}
}

func TestGroupingMalformedAnswerCrashes(t *testing.T) {
	evaluator, reply, _ := conceptRequestEvaluator(t)
	for _, answer := range []string{
		`{"assignments":[]}`,
		`{"assignments":[{"index":2,"concept":"square","score":9,"runner_up":"halve","runner_up_score":8}]}`,
		`{"assignments":[{"index":1,"concept":"square","score":8,"runner_up":"halve","runner_up_score":9}]}`,
		`{"assignments":[{"index":1,"concept":"square","score":9,"runner_up":"square","runner_up_score":8}]}`,
		`{"assignments":[{"index":1,"concept":"unknown","score":9,"runner_up":"halve","runner_up_score":8}]}`,
		`{"assignments":[{"index":1,"concept":"square","score":9,"runner_up":"halve","runner_up_score":8},` +
			`{"index":1,"concept":"square","score":9,"runner_up":"halve","runner_up_score":8}]}`,
		`{"assignments":[`,
	} {
		reply.response = answer
		item := &template.ScenarioCheckData{ID: groupTestSuite,
			Phrases: []catalogue.Step{{Block: catalogue.BlockWhen, Text: linkTestStepText}}}

		_, err := evaluator.EvaluateOne(t.Context(), item, item.ID, groupTestPrompt(9), t.TempDir())
		if err == nil {
			t.Errorf("invalid answer passed: %s", answer)
		}
	}
}

func groupTestPrompt(threshold int) checklist.PromptWithContext {
	return checklist.PromptWithContext{Index: 1, Prompt: checklist.Prompt{Question: "Group",
		With: map[string]any{"concepts": "group", "threshold": threshold}}}
}

func TestGroupingSingleConceptRequiresExplicitNullRunnerUp(t *testing.T) {
	evaluator, reply, _ := conceptRequestEvaluator(t)
	item := &template.ScenarioCheckData{ID: groupTestSuite, Phrases: []catalogue.Step{
		{Block: catalogue.BlockThen, Text: "36 is displayed"},
	}}
	reply.response = `{"assignments":[{"index":1,"concept":"result","score":10,"runner_up":null,"runner_up_score":0}]}`

	result, err := evaluator.EvaluateOne(t.Context(), item, item.ID, groupTestPrompt(9), t.TempDir())
	if err != nil || result.Status != checklist.StatusPass || len(item.ConceptLinks) != 1 {
		t.Fatalf("single concept: %+v, %v", result, err)
	}

	reply.response = `{"assignments":[{"index":1,"concept":"result","score":10,"runner_up_score":0}]}`

	_, err = evaluator.EvaluateOne(t.Context(), item, item.ID, groupTestPrompt(9), t.TempDir())
	if err == nil {
		t.Fatal("omitted runner-up must crash even with only one concept")
	}
}
