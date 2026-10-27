// Package suite_test is the fixture host project's test layer. It is
// present, runnable, and holds a failing test — so that "no failure was
// reported" cannot be mistaken for "there was nothing to find". The
// layer's command names a binary that does not exist, so the engine
// must fail to spawn it rather than run this package some other way.
package suite_test

import "testing"

// TestRed fails unconditionally. Any run that reaches it found some
// runner other than the one the spec declared.
func TestRed(t *testing.T) {
	t.Parallel()
	t.Fatal("TestRed ran: something other than the spec's declared command executed this suite")
}
