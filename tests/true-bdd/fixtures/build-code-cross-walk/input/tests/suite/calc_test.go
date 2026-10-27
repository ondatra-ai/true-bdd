package suite_test

import (
	"fixture/buildcodefix/services/calc"
	"testing"
)

func TestAVersion(t *testing.T) {
	if got := calc.Version(); got != 2 {
		t.Fatalf("Version() = %d, want 2", got)
	}
}

func TestBCompatibility(t *testing.T) {
	if calc.Version() != 1 && !calc.Compatible() {
		t.Fatal("version 2 requires compatibility support")
	}
}
