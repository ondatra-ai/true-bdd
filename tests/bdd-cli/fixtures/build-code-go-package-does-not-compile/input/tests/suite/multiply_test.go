package suite_test

import (
	"testing"

	"fixture/buildcodecompile/services/calc"
)

// TestMultiplyMultipliesItsArguments calls calc.Multiply, which
// production source does not declare, so this package does not compile.
// The fix loop has to declare it in services/calc, never edit this file.
func TestMultiplyMultipliesItsArguments(t *testing.T) {
	got := calc.Multiply(2, 3)
	if got != 6 {
		t.Fatalf("Multiply(2, 3) = %d, want 6", got)
	}
}
