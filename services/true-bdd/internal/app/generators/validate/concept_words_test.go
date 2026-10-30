package validate_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

func TestWordsJudgeOnlyLinkedMeaningsAndRejectIncompleteAnswers(t *testing.T) {
	evaluator, reply, _ := conceptRequestEvaluator(t)
	item := &template.ScenarioCheckData{ID: linkTestConcept, Phrases: []catalogue.Step{
		{Block: catalogue.BlockWhen, Text: "square six"},
	}}
	reply.response = `{"steps":{"1":{"scores":{"square":10,"halve":0},"reason":"Square.","other_block_concept":null}}}`

	_, err := evaluator.EvaluateOne(context.Background(), item, item.ID, checklist.PromptWithContext{
		Prompt: checklist.Prompt{Question: "Link", With: map[string]any{conceptsParameter: "link"}},
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	prompt := checklist.PromptWithContext{Prompt: checklist.Prompt{
		Question: "Judge words", With: map[string]any{conceptsParameter: "words"},
	}}

	for _, test := range []struct {
		response string
		pass     bool
	}{
		{`{"concepts":{"when:square":{"undeclared_words":[],"reason":"Plain English."}}}`, true},
		{`{"concepts":{"when:square":{"undeclared_words":["settlement"],"reason":"A domain operation."}}}`, false},
		{`{"concepts":{}}`, false},
		{`{"concepts":{"when:square":{"reason":"No list."}}}`, false},
		{`{"concepts":{"when:square":{"undeclared_words":null,"reason":"No list."}}}`, false},
		{`{"concepts":{"when:square":{"undeclared_words":[],"reason":""}}}`, false},
		{``, false},
		{`not JSON`, false},
		{`{"concepts":{"when:square":{"undeclared_words":[],"reason":"Plain English."},` +
			`"when:halve":{"undeclared_words":[],"reason":"Extra."}}}`, false},
	} {
		reply.response = test.response

		result, evalErr := evaluator.EvaluateOne(context.Background(), item, item.ID, prompt, t.TempDir())
		if evalErr != nil {
			t.Fatal(evalErr)
		}

		if (result.Status == checklist.StatusPass) != test.pass {
			t.Errorf("%s: %+v", test.response, result)
		}

		if !strings.Contains(reply.schema, `"when:square"`) || strings.Contains(reply.schema, `"when:halve"`) {
			t.Errorf("wrong concepts: %s", reply.schema)
		}

		if strings.Contains(reply.user, "private-architecture") {
			t.Fatal("word check read architecture")
		}
	}
}
