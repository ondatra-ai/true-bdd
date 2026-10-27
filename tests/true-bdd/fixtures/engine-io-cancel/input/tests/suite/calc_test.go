package suite_test

import (
	"fixture.test/buildcodefix/services/calc"
	"testing"
)

func TestA(t *testing.T) {
	if calc.A() != 1 {
		t.Fatal("A needs one")
	}
}
func TestB(t *testing.T) {
	if calc.B() != 2 {
		t.Fatal("B needs two")
	}
}
