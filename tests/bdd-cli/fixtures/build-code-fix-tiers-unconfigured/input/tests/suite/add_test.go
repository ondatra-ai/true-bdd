package suite_test

import (
	"testing"

	"fixture/buildcodetiers/services/calc"
)

// TestAddSumsItsArguments fails on the planted defect in calc.Add, so a
// `build code` that got past startup would have a failure to walk. The
// run never gets there: its checklist's tiers are not configured.
func TestAddSumsItsArguments(t *testing.T) {
	got := calc.Add(2, 3)
	if got != 5 {
		t.Fatalf("Add(2, 3) = %d, want 5", got)
	}
}
