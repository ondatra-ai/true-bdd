package runner

import (
	"errors"
	"strings"
	"testing"
)

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

const testRegistryPath = "docs/scenarios.yaml"

func TestJudgeGradedDropsScratch(t *testing.T) {
	t.Parallel()

	diff := []FileChange{
		{Path: "tmp/2026-01-01/checklist-01.txt"},
		{Path: "docs/product/stories/96.1-story.yaml"},
		{Path: "tmp/aiproxy-state/cursor-crush"},
		{Path: testRegistryPath},
	}

	got := judgeGraded(diff)

	want := []string{
		"docs/product/stories/96.1-story.yaml",
		testRegistryPath,
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

func TestWriteDiffSummaryShowsBothStates(t *testing.T) {
	t.Parallel()

	var buf strings.Builder

	writeDiffSummary(&buf, []FileChange{
		{Path: testRegistryPath, Kind: "modified", Before: []byte("seed text"), After: []byte("new text")},
		{Path: "docs/gone.yaml", Kind: "deleted", Before: []byte("removed text")},
		{Path: "docs/new.yaml", Kind: "created", After: []byte("fresh text")},
	})

	out := buf.String()

	for _, want := range []string{"seed text", "new text", "removed text", "fresh text"} {
		if !strings.Contains(out, want) {
			t.Errorf("diff summary is missing %q:\n%s", want, out)
		}
	}

	if strings.Contains(out, "omitting before-content") {
		t.Error("deleted files must show what was removed, not a placeholder")
	}
}

// The whole point of the schema: a verdict is read as data, so a reply
// that reasons its way through "FAIL … PASS" can no longer be scraped
// into the wrong ruling.
func TestDecodeJudgeVerdictsPasses(t *testing.T) {
	t.Parallel()

	payload := `{"verdicts":[{"rule":1,"verdict":"pass","reason":"the node survived"}]}`

	pass, reason, err := decodeJudgeVerdicts(payload, 1)
	if err != nil {
		t.Fatalf("decodeJudgeVerdicts: %v", err)
	}

	if !pass || reason != "" {
		t.Errorf("decodeJudgeVerdicts = (%v, %q), want (true, \"\")", pass, reason)
	}
}

// A failing rule names itself, so a flip is diagnosable without reading
// prose: which rule, and why.
func TestDecodeJudgeVerdictsNamesEveryFailingRule(t *testing.T) {
	t.Parallel()

	payload := `{"verdicts":[
		{"rule":1,"verdict":"pass","reason":"fine"},
		{"rule":2,"verdict":"fail","reason":"the settings page lost its account section"}
	]}`

	pass, reason, err := decodeJudgeVerdicts(payload, 2)
	if err != nil {
		t.Fatalf("decodeJudgeVerdicts: %v", err)
	}

	if pass {
		t.Error("decodeJudgeVerdicts passed a run with a failing rule")
	}

	if !strings.Contains(reason, "rule 2:") || !strings.Contains(reason, "account section") {
		t.Errorf("reason = %q, want it to name rule 2 and carry its reason", reason)
	}
}

// A rule the model never ruled on is malformed, not a silent pass — the
// failure mode that let a whole clause go ungraded.
func TestDecodeJudgeVerdictsRejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload string
		clauses int
	}{
		{"not json", "PASS", 1},
		{"empty", "", 1},
		{"fewer verdicts than clauses", `{"verdicts":[{"rule":1,"verdict":"pass","reason":"x"}]}`, 2},
		{"more verdicts than clauses", `{"verdicts":[
			{"rule":1,"verdict":"pass","reason":"x"},
			{"rule":2,"verdict":"pass","reason":"y"}]}`, 1},
		{"rule out of range", `{"verdicts":[{"rule":7,"verdict":"pass","reason":"x"}]}`, 1},
		{"repeated rule", `{"verdicts":[
			{"rule":1,"verdict":"pass","reason":"x"},
			{"rule":1,"verdict":"pass","reason":"y"}]}`, 2},
		{"unknown ruling", `{"verdicts":[{"rule":1,"verdict":"maybe","reason":"x"}]}`, 1},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := decodeJudgeVerdicts(testCase.payload, testCase.clauses)
			if !errors.Is(err, ErrJudgeMalformedResponse) {
				t.Fatalf("decodeJudgeVerdicts error = %v, want %v", err, ErrJudgeMalformedResponse)
			}
		})
	}
}
