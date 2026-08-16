// Package suite_test is the fixture host project's test layer. It is
// present, runnable, and holds a failing test — so that "no failure was
// reported" cannot be mistaken for "there was nothing to find". The
// spec is incomplete, so the engine must refuse before it ever runs
// this package.
package suite_test

import "testing"

// TestRed fails unconditionally. Any run that reaches it has walked a
// layer whose command the spec never finished declaring.
func TestRed(t *testing.T) {
	t.Parallel()
	t.Fatal("TestRed ran: the engine walked a layer with an incomplete commands: block")
}
