// Package suite_test is the fixture host project's test layer. Its one
// test fails for a reason no production edit could fix, which is the
// point: the walk must end NOT converged rather than looping.
package suite_test

import "testing"

// TestUnfixable states a contradiction. `build code` without --fix
// cannot even attempt a repair, so this failure survives the walk and
// the CLI must exit non-zero because of it.
func TestUnfixable(t *testing.T) {
	t.Parallel()
	t.Fatal("this test cannot pass: the walk must end not-converged")
}
