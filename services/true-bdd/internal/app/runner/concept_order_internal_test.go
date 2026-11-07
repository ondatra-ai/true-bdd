package runner_test

import (
	"reflect"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
)

func TestRunnerPreservesWrittenChecklistOrder(t *testing.T) {
	var calls []string

	spec := confirmationSpec(t, buildCodeCommand, applyAnswer{validationCalls: &calls})
	spec.Fix = false

	writeConfirmationFile(t, "checklists/test.yaml", `sections:
- id: written
  validation_prompts:
  - Q: {prompt: First question}
  - Q: {code: first-alphabetically}
  - Q: {prompt: Another question}
  - Q: {code: last-alphabetically}
`)

	spec.Checks = checks.Set[bool]{}
	for _, name := range []string{"last-alphabetically", "first-alphabetically"} {
		spec.Checks[name] = checks.Check[bool]{Run: func(_ bool, _ checks.Input) (checks.Verdict, error) {
			calls = append(calls, name)

			return checks.Verdict{Pass: true}, nil
		}}
	}

	err := runner.Run(t.Context(), spec)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(calls, []string{"prompt", "first-alphabetically", "prompt", "last-alphabetically"}) {
		t.Fatalf("question order = %v", calls)
	}
}
