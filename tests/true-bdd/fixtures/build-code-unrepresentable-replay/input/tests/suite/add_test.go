package suite_test

import (
	"testing"

	"fixture/buildcodefix/services/calc"
)

// TestAddSumsItsArguments is the target of the fix loop: it fails on the
// planted defect in calc.Add and passes once the operator is corrected.
//
// Deliberately trivial and NOT parallel. Its failure output is rendered
// verbatim into the validation prompt, so it lands in the recorded
// request hash — extra timing lines or interleaved === PAUSE / === CONT
// markers would make the recording fragile for no gain.
func TestAddSumsItsArguments(t *testing.T) {
	got := calc.Add(2, 3)
	if got != 5 {
		t.Fatalf("Add(2, 3) = %d, want 5", got)
	}
}
