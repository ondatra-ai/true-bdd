package validate_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// linkAnswer scores a step by its text, as the model would whichever turn asked.
func linkAnswer(key, user string) string {
	var payload struct {
		Steps []struct {
			Key  string `json:"key"`
			Text string `json:"text"`
		} `json:"steps"`
	}

	_ = json.Unmarshal([]byte(user), &payload)

	for _, step := range payload.Steps {
		switch {
		case step.Key != key:
		case strings.Contains(step.Text, "squared"):
			return `{"scores":{"square":10,"halve":0},"reason":"Squares.","other_block_concept":null}`
		case strings.Contains(step.Text, "halved"):
			return `{"scores":{"square":0,"halve":10},"reason":"Halves.","other_block_concept":null}`
		default:
			return `{"scores":{"result":10},"reason":"Shows the result.","other_block_concept":null}`
		}
	}

	return `{}`
}

func linkScenarios() []*template.ScenarioCheckData {
	return []*template.ScenarioCheckData{
		{ID: "scenario-square", Service: linkTestService, Description: "secret description", CheckedSteps: []catalogue.Step{
			{Block: catalogue.BlockWhen, Text: squaredStep}, {Block: catalogue.BlockThen, Text: displayedStep},
			{Block: catalogue.BlockThen, Text: "judge: the answer reads well"},
		}},
		{ID: sharedBothID, Service: linkTestService, CheckedSteps: []catalogue.Step{
			{Block: catalogue.BlockWhen, Text: squaredStep}, {Block: catalogue.BlockWhen, Text: "6 is halved"},
		}},
	}
}

func TestSharedLinkJudgesEachDistinctStepOncePerWalkAndReadsAsTodaysLink(t *testing.T) {
	reply := newSharedReply(0, linkAnswer)
	evaluator, _ := conceptRequestEvaluatorWith(t, reply)
	shared := linkScenarios()
	prompt := sharedPrompt(linkJudgement)
	prompt.Prompt.With["batch"] = 2

	evaluator.StartWalk(walkSubjects(shared...))
	results := evaluateTogether(t, evaluator, prompt, shared...)

	turns := reply.judged()
	if len(turns) != 2 || len(turns[0].keys)+len(turns[1].keys) != 3 {
		t.Fatalf("three distinct steps in two balanced batches, got %+v", turns)
	}

	for _, turn := range turns {
		if strings.Contains(turn.user, "scenario-") || strings.Contains(turn.user, "secret description") {
			t.Fatalf("a shared link turn reads steps, not a Scenario: %s", turn.user)
		}
	}

	alone := linkScenarios()
	today := evaluateTogether(t, evaluator, checklist.PromptWithContext{Index: 3, SectionID: "concept",
		Prompt: checklist.Prompt{Question: "Judge link", With: map[string]any{conceptsParameter: linkJudgement}}},
		alone...)

	assertReadsAsToday(t, shared, alone, results, today)
}

func assertReadsAsToday(t *testing.T, shared, alone []*template.ScenarioCheckData,
	results, today map[string]checklist.ValidationResult,
) {
	t.Helper()

	for index, subject := range shared {
		got, want := results[subject.ID], today[subject.ID]
		if got.Status != checklist.StatusPass || got.Status != want.Status || !slices.Equal(got.Context, want.Context) {
			t.Errorf("%s: shared %s %q, today %s %q", subject.ID, got.Status, got.Context, want.Status, want.Context)
		}

		if !subject.ConceptLinkPassed || !slices.Equal(subject.ConceptLinks, alone[index].ConceptLinks) {
			t.Errorf("%s: shared links %+v, today %+v", subject.ID, subject.ConceptLinks, alone[index].ConceptLinks)
		}
	}
}
