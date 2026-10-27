// Package suite_test is the fixture host project's test layer. It is
// present, runnable, and holds a failing test — which is the whole
// point: the layer's command omits `-json`, so a run that went ahead
// would read this failure as prose, find nothing in it, and report
// success. The failure has to exist for that false green to be possible.
package suite_test

import "testing"

// TestRed fails unconditionally. Any run that reaches it is one the
// engine let proceed with output it cannot parse.
func TestRed(t *testing.T) {
	t.Parallel()
	t.Fatal("TestRed ran: the engine executed a command whose output it cannot parse")
}
