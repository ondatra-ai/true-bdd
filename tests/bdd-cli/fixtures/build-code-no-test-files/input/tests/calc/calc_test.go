package calc_test

import (
	"testing"

	"fixture/buildmember/tests/calc/helper"
)

func TestDouble(t *testing.T) {
	if got := helper.Double(3); got != 6 {
		t.Fatalf("Double(3) = %d, want 6", got)
	}
}
