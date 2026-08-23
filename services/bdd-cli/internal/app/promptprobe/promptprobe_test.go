package promptprobe_test

// prompt-probe command behavior: it emits choice → clarify → freetext
// prompts through the real collector, one stdin answer per prompt
// (freetext ends on a blank line); the three prompts carry distinct ids.

import (
	"strings"
	"testing"
)

// ProbeRun is what one `prompt-probe` execution observably produced: the
// emitted prompt kinds (in order), their ids, and the answers the collector
// recorded from stdin.
type ProbeRun struct {
	PromptKinds []string
	PromptIDs   []string
	Recorded    []string
}

func TestPromptProbeEmitsChoiceClarifyFreetextInOrder(t *testing.T) {
	// One answer per prompt: choice "1", clarify "2", then multiline freetext
	// terminated by a blank line.
	stdin := strings.NewReader("1\n2\nrefinement feedback\n\n")

	run := runProbe(t, stdin)

	want := []string{"choice", "clarify", "freetext"}
	if len(run.PromptKinds) != len(want) {
		t.Fatalf("emitted %v, want %v", run.PromptKinds, want)
	}

	for i, kind := range want {
		if run.PromptKinds[i] != kind {
			t.Fatalf("prompt %d: %q, want %q", i, run.PromptKinds[i], kind)
		}
	}
}

func TestPromptProbeEmitsDistinctPromptIDs(t *testing.T) {
	run := runProbe(t, strings.NewReader("1\n2\nfeedback\n\n"))

	if len(run.PromptIDs) != 3 {
		t.Fatalf("expected 3 prompt ids, got %d", len(run.PromptIDs))
	}

	seen := map[string]bool{}
	for _, id := range run.PromptIDs {
		if id == "" || seen[id] {
			t.Fatalf("prompt ids must be non-empty and distinct: %v", run.PromptIDs)
		}

		seen[id] = true
	}
}

func TestPromptProbeRecordsAnswersFromStdin(t *testing.T) {
	run := runProbe(t, strings.NewReader("1\n2\nrefinement feedback\n\n"))

	if len(run.Recorded) != 3 {
		t.Fatalf("expected 3 recorded answers, got %d", len(run.Recorded))
	}

	for i, ans := range run.Recorded {
		if strings.TrimSpace(ans) == "" {
			t.Fatalf("recorded answer %d is empty", i)
		}
	}

	// The freetext answer (multiline collector) carries the feedback text.
	if !strings.Contains(run.Recorded[2], "refinement feedback") {
		t.Fatalf("freetext answer must record the feedback: %q", run.Recorded[2])
	}
}
