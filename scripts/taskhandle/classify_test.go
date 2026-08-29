package taskhandle_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/taskhandle"
)

func finding(kind string) taskhandle.Finding {
	return taskhandle.Finding{Kind: kind, What: kind + " thing", Where: "a.go:1"}
}

// One unasked finding refuses the whole run, even beside gaps a fix could
// close: unasked work is a refusal on the merits, not a repair job.
func TestClassifySeparatesUnaskedWork(t *testing.T) {
	t.Parallel()

	toFix, unasked := taskhandle.Classify([]taskhandle.Finding{
		finding("missing"), finding("unasked"), finding("wrong"),
	})

	if len(unasked) != 1 {
		t.Fatalf("got %d unasked, want 1", len(unasked))
	}

	if len(toFix) != 2 {
		t.Errorf("got %d to fix, want 2 (missing and wrong)", len(toFix))
	}
}

func TestClassifyOfACleanReview(t *testing.T) {
	t.Parallel()

	toFix, unasked := taskhandle.Classify(nil)
	if len(toFix) != 0 || len(unasked) != 0 {
		t.Errorf("a clean review classified as %v / %v", toFix, unasked)
	}
}

// A kind outside the three is a gap, never silently dropped: dropping it would
// let the run merge over a finding nobody read.
func TestClassifyTreatsAnUnknownKindAsAGap(t *testing.T) {
	t.Parallel()

	toFix, unasked := taskhandle.Classify([]taskhandle.Finding{finding("smell")})
	if len(toFix) != 1 || len(unasked) != 0 {
		t.Errorf("an unknown kind gave %v / %v, want it treated as a gap", toFix, unasked)
	}
}
