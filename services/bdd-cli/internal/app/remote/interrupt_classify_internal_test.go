package remote

import (
	"context"
	"errors"
	"testing"
)

// cancelledCtx returns an already-cancelled context.
func cancelledCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	return ctx
}

// TestNaturalCompletionThenLateCancelIsNotInterrupted proves the NICE-1 fix:
// a child that completed naturally (a present result, clean exit) must not
// be reclassified interrupted just because the run ctx was cancelled afterwards.
func TestNaturalCompletionThenLateCancelIsNotInterrupted(t *testing.T) {
	t.Parallel()

	executor := &RunExecutor{
		result: childResult{present: true, outcome: outcomeConverged, finalizationOK: true},
	}

	if executor.wasInterrupted(cancelledCtx(), nil) {
		t.Fatal("a clean natural completion racing a late cancel must NOT be interrupted")
	}

	env := executor.classifyTerminal(cancelledCtx(), nil)
	if env.outcome != outcomeConverged {
		t.Fatalf("outcome = %q, want the real %q (not overwritten)", env.outcome, outcomeConverged)
	}
}

// TestRealInterruptRobustToChildExitRace proves the interrupt path is
// robust when the child dies (non-zero, no result) from the group signal
// BEFORE watchShutdown observes it — a non-clean exit still classifies interrupted, never error(no_result).
func TestRealInterruptRobustToChildExitRace(t *testing.T) {
	t.Parallel()

	executor := &RunExecutor{} // no result recorded (evaluator's claude call failed)

	if !executor.wasInterrupted(cancelledCtx(), errors.New("child exited non-zero")) {
		t.Fatal("a real interrupt (ctx cancelled, no clean result) must classify interrupted")
	}
}

// TestCausalFlagInterrupts proves the causal shutdown flag alone forces
// the interrupted classification even on a clean exit.
func TestCausalFlagInterrupts(t *testing.T) {
	t.Parallel()

	executor := &RunExecutor{result: childResult{present: true, outcome: outcomeConverged}}
	executor.interrupted.Store(true)

	if !executor.wasInterrupted(context.Background(), nil) {
		t.Fatal("the causal shutdown flag must force interrupted")
	}
}
