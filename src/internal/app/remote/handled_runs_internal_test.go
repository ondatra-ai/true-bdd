package remote

import "testing"

// TestHandledRunsBounded proves the dedup set is bounded (plan §3.6: prune
// the spike's unbounded handled-run set): once over the cap the oldest id
// is evicted, while recent ids stay remembered.
func TestHandledRunsBounded(t *testing.T) {
	t.Parallel()

	handled := newHandledRuns(3)
	handled.add("r1")
	handled.add("r2")
	handled.add("r3")

	if !handled.has("r1") || !handled.has("r3") {
		t.Fatal("recent ids should be remembered")
	}

	handled.add("r4") // evicts the oldest (r1)

	if handled.has("r1") {
		t.Fatal("oldest id should have been evicted")
	}

	if !handled.has("r2") || !handled.has("r4") {
		t.Fatal("r2 and r4 should be remembered")
	}

	handled.remove("r2")

	if handled.has("r2") {
		t.Fatal("removed id should be forgotten")
	}
}

// TestDispatchRunClaimRedelivery proves a dispatched run is handed to the
// main loop exactly once even when the server re-delivers it on successive
// polls until it is claimed (plan §3.3): the second delivery is deduped.
func TestDispatchRunClaimRedelivery(t *testing.T) {
	t.Parallel()

	agent := &Agent{handled: newHandledRuns(handledRunsCap)}
	runCh := make(chan RunSpec, 1)

	run := RunSpec{RunID: testRunID, Command: commandVersion}

	agent.dispatchRun(run, runCh)

	select {
	case got := <-runCh:
		if got.RunID != testRunID {
			t.Fatalf("dispatched run id = %q", got.RunID)
		}
	default:
		t.Fatal("expected the run to be dispatched once")
	}

	// Server re-delivers the same run before it is claimed: no second hand-off.
	agent.dispatchRun(run, runCh)

	if len(runCh) != 0 {
		t.Fatal("re-delivered run must not be dispatched twice")
	}
}
