package suite_test

import (
	"fixture/buildcodefix/services/calc"
	"testing"
)

func TestAAdd(t *testing.T) {
	if got := calc.Add(2, 3); got != 5 {
		t.Fatalf("Add(2, 3) = %d, want 5", got)
	}
}

func TestBAdd(t *testing.T) {
	if got := calc.Add(4, 3); got != 7 {
		t.Fatalf("Add(4, 3) = %d, want 7", got)
	}
}
