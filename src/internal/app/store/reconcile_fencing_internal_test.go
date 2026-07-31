package store

import "testing"

// TestReconcileInspectionIsNonMutating is the regression for the reproduced
// crash-between-kill-and-abandon window (finding 5): the OLD Reconcile marked a
// run terminal `abandoned` BEFORE the caller killed its live orphan group, so a
// crash in that gap hid a live orphan behind a terminal row and every later
// pass ignored it. Reconcile is now a NON-mutating inspection — the run stays
// visibly nonterminal until the FENCED abandon, which the caller issues ONLY
// after draining/killing the group.
func TestReconcileInspectionIsNonMutating(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	insertReconcileRun(t, store, "run-orphan", "owner-dead")

	// Owner dead, child ALIVE — the classic terminate-then-abandon case.
	plan := store.Reconcile(10, liveSet(map[int]string{reconcileChildPGID: reconcileChildID}))
	if !signalledOrphan(plan.Terminated) || !contains(plan.Abandoned, "run-orphan") {
		t.Fatalf("plan must terminate the orphan and abandon the run: %+v", plan)
	}

	// The reproduced defect: inspection must NOT have marked the run terminal.
	// A crash HERE (after planning, before the kill) must leave the run visibly
	// nonterminal so the NEXT pass re-terminates the still-live orphan.
	if runState(t, store, "run-orphan") == stateTerminal {
		t.Fatal("Reconcile must NOT abandon before the kill — the orphan would be hidden behind a terminal row (finding 5)")
	}

	// The caller kills the group FIRST, then abandons under the fencing token.
	tok, ok := store.AcquireReconcileLease("owner-dead", 20, 1000)
	if !ok {
		t.Fatal("acquire lease")
	}

	err := store.AbandonRunFenced("run-orphan", "owner-dead", tok)
	if err != nil {
		t.Fatalf("fenced abandon: %v", err)
	}

	if runState(t, store, "run-orphan") != stateTerminal {
		t.Fatalf("after the fenced abandon the run must be terminal, got %q", runState(t, store, "run-orphan"))
	}
}

// TestRenewReconcileLeaseNowPlusTTL proves the production renewal (finding 5):
// a renew extends expires_at to now+ttl (keeping the holder past the original
// expiry), only the current holder+token may renew, and a takeover after the
// RENEWED expiry mints a fresh fencing token.
func TestRenewReconcileLeaseNowPlusTTL(t *testing.T) {
	store := requireStore(t, tempDBPath(t))

	const ttl = 1000

	tok, ok := store.AcquireReconcileLease("owner-A", 100, ttl)
	if !ok {
		t.Fatal("first claimant must win")
	}

	// A renew at 500 extends the lease to 500+ttl=1500 (past the original 1100).
	if !store.RenewReconcileLease("owner-A", tok, 500) {
		t.Fatal("the current holder must renew")
	}

	// Only the current holder + token may renew.
	if store.RenewReconcileLease("owner-B", tok, 600) {
		t.Fatal("a different owner must NOT renew")
	}

	if store.RenewReconcileLease("owner-A", tok+99, 600) {
		t.Fatal("a stale fencing token must NOT renew")
	}

	// Past the ORIGINAL expiry (1100) but before the RENEWED one (1500): still
	// single-winner — a fresh acquire cannot take over.
	if _, won := store.AcquireReconcileLease("owner-B", 1200, ttl); won {
		t.Fatal("renew must keep the lease held past the original expiry")
	}

	// Past the RENEWED expiry (1500): takeover succeeds with a NEW token, and
	// the superseded holder's fenced action is rejected.
	newTok, won := store.AcquireReconcileLease("owner-B", 1600, ttl)
	if !won || newTok == tok {
		t.Fatalf("takeover after the renewed expiry must mint a new token: won=%v tok=%d old=%d", won, newTok, tok)
	}

	err := store.AbandonRunFenced("run-x", "owner-A", tok)
	if err == nil {
		t.Fatal("the superseded claimant's fenced action must be rejected")
	}
}

func runState(t *testing.T, store Store, runID string) string {
	t.Helper()

	view, ok := store.RunView(runID)
	if !ok {
		t.Fatalf("run %s not found", runID)
	}

	return view.State
}
