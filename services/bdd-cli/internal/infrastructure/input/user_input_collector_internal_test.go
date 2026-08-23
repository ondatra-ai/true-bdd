package input

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/checklist"
)

// TestAskApplyRefineOrExitMapping proves the choice enum mapping: numeric
// 1/2/3 and the literal apply/refine/exit map to the engine's actions,
// and anything else defaults to exit (plan §3.2 / A0/A2/A3).
func TestAskApplyRefineOrExitMapping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  ActionChoice
	}{
		{"1", ActionApply},
		{"apply", ActionApply},
		{"2", ActionRefine},
		{"refine", ActionRefine},
		{"3", ActionExit},
		{"exit", ActionExit},
		{"", ActionExit},
		{"garbage", ActionExit},
	}

	for _, testCase := range cases {
		t.Run(testCase.input, func(t *testing.T) {
			t.Parallel()

			collector := newUserInputCollector(strings.NewReader(testCase.input + "\n"))

			got := collector.AskApplyRefineOrExit()
			if got != testCase.want {
				t.Fatalf("input %q → %q, want %q", testCase.input, got, testCase.want)
			}
		})
	}
}

// TestAskQuestionsMapsNumberToOptionText proves the clarify mapping A3
// relies on: the browser answers with the option number and the collector
// records its text; non-numeric or out-of-range input passes through verbatim.
func TestAskQuestionsMapsNumberToOptionText(t *testing.T) {
	t.Parallel()

	options := []string{"Merge both duplicates", "Keep only the first"}

	cases := []struct {
		input string
		want  string
	}{
		{"1", "Merge both duplicates"},
		{"2", "Keep only the first"},
		{"custom answer", "custom answer"},
		{"9", "9"},
	}

	for _, testCase := range cases {
		t.Run(testCase.input, func(t *testing.T) {
			t.Parallel()

			collector := newUserInputCollector(strings.NewReader(testCase.input + "\n"))
			answers := collector.AskQuestions([]checklist.ClarifyQuestion{
				{ID: "q1", Question: "Which duplicate wins?", Options: options},
			})

			if answers["q1"] != testCase.want {
				t.Fatalf("input %q → %q, want %q", testCase.input, answers["q1"], testCase.want)
			}
		})
	}
}

// TestAskRefinementFeedbackMultiline proves the multiline transport: the
// collector reads lines until a blank line terminates the block and returns
// the joined body, leaving surplus input unread (A2's contract).
func TestAskRefinementFeedbackMultiline(t *testing.T) {
	t.Parallel()

	collector := newUserInputCollector(strings.NewReader("first line\nsecond line\n\nignored trailing line\n"))

	feedback := collector.AskRefinementFeedback()
	if feedback != "first line\nsecond line" {
		t.Fatalf("feedback = %q, want %q", feedback, "first line\nsecond line")
	}
}
