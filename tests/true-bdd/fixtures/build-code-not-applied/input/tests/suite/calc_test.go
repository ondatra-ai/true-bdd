package suite_test

import (
	"fixture/buildcodefix/services/calc"
	"testing"
)

func TestAdd(t *testing.T) {
	if got := calc.Add(2, 3); got != 5 {
		t.Fatalf("Add(2, 3) = %d, want 5", got)
	}
}
