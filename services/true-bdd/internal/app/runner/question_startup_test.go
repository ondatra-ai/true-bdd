package runner_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
)

func TestMalformedChecklistQuestionRefusesBeforeLoadingItems(t *testing.T) {
	for _, question := range []string{
		"D: tests-pass", "D: tests-pass\n    skip: true", "Q: legacy question",
		"Q: {prompt: question, code: tests-pass}", "Q: {}",
	} {
		t.Run(question, func(t *testing.T) {
			spec := confirmationSpec(t, buildCodeCommand, applyAnswer{})
			writeConfirmationFile(t, "checklists/test.yaml",
				"sections:\n- id: verdict\n  validation_prompts:\n  - "+question+"\n    F: repair\n")

			spec.LoadItems = nil

			err := runner.Run(t.Context(), spec)
			if err == nil {
				t.Fatal("malformed question reached the walk")
			}

			for _, want := range []string{"test/verdict #1", "Q: {prompt:", "Q: {code:"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("startup error %v omits %q", err, want)
				}
			}
		})
	}
}
