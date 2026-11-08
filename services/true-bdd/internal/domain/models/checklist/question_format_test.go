package checklist_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"gopkg.in/yaml.v3"
)

func TestChecklistQuestionFormat(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, source, question, check string
		refused                       bool
	}{
		{name: "prompt preserves bytes", source: "Q:\n  prompt: |\n    Is it correct\n\n", question: "Is it correct\n"},
		{name: "code", source: "Q: {code: tests-pass}\nwith: {suite: calc}", check: "tests-pass"},
		{name: "legacy code", source: "D: tests-pass", refused: true},
		{name: "legacy scalar", source: "Q: Is it correct", refused: true},
		{name: "both", source: "Q: {prompt: correct, code: tests-pass}", refused: true},
		{name: "neither", source: "Q: {}", refused: true},
		{name: "missing", source: "F: repair", refused: true},
		{name: "unknown option", source: "Q: {prompt: correct, other: ignored}", refused: true},
		{name: "code model", source: "Q: {code: tests-pass}\nmodel: high", refused: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var prompt checklist.Prompt

			err := yaml.Unmarshal([]byte(test.source), &prompt)
			if err != nil {
				t.Fatal(err)
			}

			err = prompt.ShapeError()
			if test.refused {
				if err == nil {
					t.Fatal("invalid question accepted")
				}

				if test.name != "code model" {
					assertReplacement(t, err)
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if prompt.Question != test.question || prompt.Check != test.check {
				t.Fatalf("question=%q check=%q", prompt.Question, prompt.Check)
			}
		})
	}
}

func assertReplacement(t *testing.T, err error) {
	t.Helper()

	for _, option := range []string{"Q: {prompt:", "Q: {code:"} {
		if !strings.Contains(err.Error(), option) {
			t.Fatalf("replacement missing: %v", err)
		}
	}
}
