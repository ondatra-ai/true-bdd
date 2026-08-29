package taskhandle_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/taskhandle"
)

// The cap is shared by the two recoveries, so five units bought at step 5 are
// five units step 6 no longer has.
func TestBudgetIsOneCapAcrossBothRecoveries(t *testing.T) {
	t.Parallel()

	budget := taskhandle.NewBudget()

	for attempt := 1; attempt <= 5; attempt++ {
		got, err := budget.Spend("gates red")
		if err != nil {
			t.Fatalf("attempt %d refused: %v", attempt, err)
		}

		if got != attempt {
			t.Errorf("attempt reported as %d, want %d", got, attempt)
		}
	}

	if got := budget.Spent(); got != 5 {
		t.Errorf("Spent() = %d, want 5", got)
	}

	_, err := budget.Spend("the review kept finding gaps")
	if err == nil {
		t.Fatal("the sixth attempt was allowed")
	}

	if !taskhandle.IsDecline(err) {
		t.Errorf("the sixth gave %T, want a decline — the step text wins over Halting", err)
	}

	if !strings.Contains(err.Error(), "the review kept finding gaps") {
		t.Errorf("the decline does not say why: %q", err)
	}
}

func TestAFreshBudgetHasSpentNothing(t *testing.T) {
	t.Parallel()

	if got := taskhandle.NewBudget().Spent(); got != 0 {
		t.Errorf("Spent() = %d, want 0", got)
	}
}
