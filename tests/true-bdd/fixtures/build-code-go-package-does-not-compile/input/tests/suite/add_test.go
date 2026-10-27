package suite_test

import (
	"testing"

	"fixture/buildcodecompile/services/calc"
)

// TestAddSumsItsArguments would pass on its own, but it shares a package
// with a test that does not compile, so it never runs until that one
// does: the package is one failure, not one per test.
func TestAddSumsItsArguments(t *testing.T) {
	got := calc.Add(2, 3)
	if got != 5 {
		t.Fatalf("Add(2, 3) = %d, want 5", got)
	}
}
