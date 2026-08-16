// Package suite_test is the fixture host project's test layer. It is
// present, runnable, and holds a failing test — which is the whole
// point: the layer's replay command is `''`, so the run must be refused
// before any test runner is spawned. This failure is the decoy that
// makes the refusal falsifiable. If the engine ever fell back to some
// substituted invocation, it would run this test, find the failure, and
// the log would carry a "Spawning test runner" record. A run that
// spawned nothing did not fall back.
package suite_test

import "testing"

// TestRed fails unconditionally. Any run that reaches it is one the
// engine let proceed with a command that names no program.
func TestRed(t *testing.T) {
	t.Parallel()
	t.Fatal("TestRed ran: the engine spawned a runner for a command whose only argument is empty")
}
