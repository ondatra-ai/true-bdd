// Package suite_test is the fixture host project's test tree. It is
// present, runnable, and holds a failing test — so that "no failure was
// reported" cannot be mistaken for "there was nothing to find". The
// suite names a service that does not exist, so the engine must refuse
// before it ever runs this package.
package suite_test

import "testing"

// TestRed fails unconditionally. Any run that reaches it has walked a
// suite whose failures no fix could be pointed at.
func TestRed(t *testing.T) {
	t.Parallel()
	t.Fatal("TestRed ran: the engine walked a suite naming an undeclared service")
}
