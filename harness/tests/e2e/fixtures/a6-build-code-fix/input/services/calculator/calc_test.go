package calculator

import "testing"

// TestAdd is the failing test build-code discovers. The fix loop must
// make Add(2, 3) == 5 by editing calc.go — never this file.
func TestAdd(t *testing.T) {
	if got := Add(2, 3); got != 5 {
		t.Fatalf("Add(2, 3) = %d, want 5", got)
	}
}
