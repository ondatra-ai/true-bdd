// Package suite_test is the fixture host project's test layer. Two
// tests, one passing and one failing, in a single package — so which of
// them runs is decided by the command the architectural spec declares
// and by nothing else.
package suite_test

import "testing"

// TestGreen is the test the spec's command selects.
func TestGreen(t *testing.T) {
	t.Parallel()
}

// TestRed is the control, and the whole point of the fixture. The
// spec's replay command filters it out with `-run '^TestGreen$'`, so it
// runs only when the engine substitutes an invocation of its own, drops
// part of the one it was given, or reaches for the wrong mode's
// command. Any of those turns this into a discovered failure and the
// fixture goes red.
func TestRed(t *testing.T) {
	t.Parallel()
	t.Fatal("TestRed ran: the command the engine used is not the one the spec declared")
}
