package runner

import (
	"errors"
	"strings"
	"testing"
)

func TestParseJudgeVerdict(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		response   string
		wantPass   bool
		wantReason string
	}{
		{
			name:     "the bare verdict the rubric asks for",
			response: "PASS",
			wantPass: true,
		},
		{
			name:       "a bare failure carries its reason",
			response:   "FAIL: the registry was modified",
			wantReason: "the registry was modified",
		},
		{
			// The case that failed real fixtures: the judge works through
			// the rubric point by point and answers at the END.
			name: "a reasoned reply ending in the verdict",
			response: "Looking at the diff against the specification:\n\n" +
				"1. Created file at a valid path — satisfies requirement 1.\n" +
				"2. References scenario id INT-900 — satisfies requirement 2.\n\n" +
				"All conditions are satisfied.\n\nPASS",
			wantPass: true,
		},
		{
			name:       "a reasoned reply ending in a failure",
			response:   "The spec file is missing.\n\nFAIL: no test references INT-900",
			wantReason: "no test references INT-900",
		},
		{
			// The conclusion is what the reply means, so a verdict named
			// while reasoning must not outrank the one at the end.
			name:     "a verdict quoted mid-reasoning loses to the conclusion",
			response: "Requirement 2 would be FAIL: if the id were absent, but it is present.\n\nPASS",
			wantPass: true,
		},
		{
			name:     "emphasis and a trailing stop are decoration, not content",
			response: "Everything checks out.\n\n**PASS**.",
			wantPass: true,
		},
		{
			name:       "trailing prose after the verdict does not hide it",
			response:   "FAIL: the story file was rewritten\n\nSee the diff for details.",
			wantReason: "the story file was rewritten",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			pass, reason, err := parseJudgeVerdict(testCase.response)
			if err != nil {
				t.Fatalf("parseJudgeVerdict: %v", err)
			}

			if pass != testCase.wantPass {
				t.Fatalf("pass = %v, want %v", pass, testCase.wantPass)
			}

			if reason != testCase.wantReason {
				t.Fatalf("reason = %q, want %q", reason, testCase.wantReason)
			}
		})
	}
}

// Tolerating prose must not tip into guessing: a reply that never states
// a verdict is an error, not a pass.
func TestParseJudgeVerdictRejects(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		response string
		wantErr  error
	}{
		{
			name:     "no verdict anywhere",
			response: "I could not tell whether the run was correct.",
			wantErr:  ErrJudgeMalformedResponse,
		},
		{
			name:     "the word appears only inside prose",
			response: "The first two checks pass and the third passes too.",
			wantErr:  ErrJudgeMalformedResponse,
		},
		{
			name:     "empty response",
			response: "",
			wantErr:  ErrJudgeMalformedResponse,
		},
		{
			name:     "a failure with no reason breaks the contract",
			response: "FAIL:",
			wantErr:  ErrJudgeEmptyFailReason,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := parseJudgeVerdict(testCase.response)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("err = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}

// The clauses a scenario collected become the numbered rubric the judge
// was always handed. Checked because nothing else does: replay never calls
// the judge, so a rendering bug would only ever surface in a live run.
func TestBuildJudgeUserPromptRendersClauses(t *testing.T) {
	t.Parallel()

	prompt := buildJudgeUserPrompt(JudgeRequest{
		Cmd:     "us refine 96.1",
		Clauses: []string{"the clauses keep their meaning", "no numeric threshold changed"},
	})

	for _, want := range []string{
		"true-bdd us refine 96.1",
		"1. the clauses keep their meaning",
		"2. no numeric threshold changed",
		"## Tolerances",
		"Rules 1..2",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt is missing %q\n---\n%s", want, prompt)
		}
	}
}

// A judge told only "here are two rules" re-derives the rest from the diff
// and fails runs for files the suite already approved. The scope paragraph
// is what prevents that, so it has to actually say so.
func TestBuildJudgeUserPromptDisclaimsWhatWasCheckedMechanically(t *testing.T) {
	t.Parallel()

	prompt := buildJudgeUserPrompt(JudgeRequest{Cmd: "us refine 96.1", Clauses: []string{"one"}})

	// Whitespace-normalised: the scope paragraph is prose and wraps, so a
	// literal match would break on a reflow that changed nothing.
	flat := strings.Join(strings.Fields(prompt), " ")

	for _, want := range []string{"exit code", "already been checked mechanically"} {
		if !strings.Contains(flat, want) {
			t.Errorf("prompt must tell the judge what is not its job; missing %q", want)
		}
	}
}

func TestJudgeGradedDropsScratch(t *testing.T) {
	t.Parallel()

	diff := []FileChange{
		{Path: "tmp/2026-01-01/checklist-01.txt"},
		{Path: "docs/product/stories/96.1-story.yaml"},
		{Path: "tmp/aiproxy-state/cursor-crush"},
		{Path: "docs/scenarios.yaml"},
	}

	got := judgeGraded(diff)

	want := []string{
		"docs/product/stories/96.1-story.yaml",
		"docs/scenarios.yaml",
	}

	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}

	for index, path := range want {
		if got[index].Path != path {
			t.Errorf("position %d = %q, want %q", index, got[index].Path, path)
		}
	}
}
