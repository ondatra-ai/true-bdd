package validate_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

const halvedStep = "6 is halved"

const boundArchitecture = "architecture:\n  services:\n  - name: calc\n    public_interface:\n" +
	"      kind: ui\n      elements: [{page: Calc, role: button, name: Square}]\n  concepts:\n" +
	"  - {concept: square, block: when, service: calc, does: Press Square.}\n" +
	"  - {concept: halve, block: when, service: calc, does: Press Halve.}\n" +
	"  - {concept: result, block: then, service: calc, does: Read the display.}\n"

// linkAnswer scores a step by its text.
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

func bindingAnswer(key, user string) string {
	switch {
	case !strings.Contains(key, ":"):
		return linkAnswer(key, user)
	case key == "when:halve":
		return `{"unambiguous":false,"reason":"Which number?"}`
	default:
		return `{"unambiguous":true,"reason":"Clear."}`
	}
}

func TestBindingsJudgeEachBindingOncePerWalkWithEveryStepLinkingIt(t *testing.T) {
	reply := newSharedReply(0, bindingAnswer)
	evaluator, _ := conceptRequestEvaluatorWith(t, reply)

	err := disk.Write("private-architecture.yaml", []byte(boundArchitecture), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	square := &template.ScenarioCheckData{ID: "scenario-square", Service: linkTestService, CheckedSteps: []catalogue.Step{
		{Block: catalogue.BlockWhen, Text: squaredStep}, {Block: catalogue.BlockThen, Text: displayedStep},
	}}
	both := &template.ScenarioCheckData{ID: sharedBothID, Service: linkTestService, CheckedSteps: []catalogue.Step{
		{Block: catalogue.BlockWhen, Text: "7 is squared"}, {Block: catalogue.BlockWhen, Text: halvedStep},
	}}
	unlinked := &template.ScenarioCheckData{ID: "scenario-unlinked", Service: linkTestService,
		CheckedSteps: []catalogue.Step{{Block: catalogue.BlockWhen, Text: "8 is cubed"}}}

	evaluator.StartWalk()
	evaluateTogether(t, evaluator, conceptPrompt(linkJudgement), square, both, unlinked)
	results := evaluateTogether(t, evaluator, conceptPrompt("bindings"), square, both, unlinked)

	turns := reply.judged()[3:]
	if len(reply.judged()) != 6 {
		t.Fatalf("a link turn per Scenario and one turn per linked binding, got %+v", reply.judged())
	}

	for _, turn := range turns {
		assertBindingProductPath(t, turn.user)

		if len(turn.keys) != 1 || strings.Contains(turn.user, "8 is cubed") {
			t.Fatalf("a binding turn judges one binding with the walk's passing links only: %+v", turn)
		}

		squares := strings.Contains(turn.user, squaredStep) && strings.Contains(turn.user, "7 is squared")
		if (turn.keys[0] == "when:square") != squares || strings.Contains(turn.user, "scenario-") {
			t.Fatalf("the square binding reads every step linking it, and no Scenario: %+v", turn)
		}
	}

	assertContext(t, results["scenario-square"], checklist.StatusPass, "then:result: Clear.", "when:square: Clear.")
	assertContext(t, results[sharedBothID], checklist.StatusFail, "when:halve: Which number?",
		"when:halve: binding is ambiguous", "when:square: Clear.")
	assertContext(t, results["scenario-unlinked"], checklist.StatusFail,
		"binding judgement requires this walk's successful link")
}

func assertBindingProductPath(t *testing.T, user string) {
	t.Helper()

	if !strings.Contains(user, `"product_path": "product.yaml"`) || strings.Contains(user, "glossary:") {
		t.Fatalf("binding must read complete product by path: %s", user)
	}
}

func TestFixedBindingIsJudgedAgainWithTheWalksSharedSteps(t *testing.T) {
	repaired := false
	reply := newSharedReply(0, func(key, user string) string {
		if repaired && strings.Contains(key, ":") {
			return `{"unambiguous":true,"reason":"The binding was corrected."}`
		}

		return bindingAnswer(key, user)
	})
	evaluator, _ := conceptRequestEvaluatorWith(t, reply)

	err := disk.Write("private-architecture.yaml", []byte(boundArchitecture), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	first := &template.ScenarioCheckData{ID: "first", Service: linkTestService, CheckedSteps: []catalogue.Step{
		{Block: catalogue.BlockWhen, Text: halvedStep},
	}}
	second := &template.ScenarioCheckData{ID: "second", Service: linkTestService, CheckedSteps: []catalogue.Step{
		{Block: catalogue.BlockWhen, Text: "8 is halved"},
	}}

	evaluator.StartWalk()
	evaluateTogether(t, evaluator, conceptPrompt(linkJudgement), first, second)
	before := evaluateTogether(t, evaluator, conceptPrompt("bindings"), first, second)
	assertContext(t, before["first"], checklist.StatusFail,
		"when:halve: Which number?", "when:halve: binding is ambiguous")

	repaired = true

	evaluator.InvalidateJudgements()
	after := evaluateTogether(t, evaluator, conceptPrompt("bindings"), first, second)
	assertContext(t, after["first"], checklist.StatusPass, "when:halve: The binding was corrected.")

	turns := reply.judged()
	if len(turns) != 4 {
		t.Fatalf("want two links and two binding judgements, got %d", len(turns))
	}

	for _, text := range []string{halvedStep, "8 is halved"} {
		if !strings.Contains(turns[3].user, text) {
			t.Fatalf("fixed binding lost the shared step %q", text)
		}
	}
}
