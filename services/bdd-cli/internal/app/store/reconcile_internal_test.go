package store

// Boot reconciliation matrix (plan §1.6): for each nonterminal run, validate
// owner pid+start_identity and child pgid+start_identity; both dead ⇒
// abandoned; owner dead + child ALIVE ⇒ verify identity, terminate the
// group, abandon; a LIVE sibling's run is untouched; NEVER infer liveness
// from a PID alone (PID reuse ⇒ identity mismatch ⇒ treated dead; PGID reuse
// with an absent leader ⇒ no signal). Plus the durable, FENCED single-winner
// reconciliation lease.

import (
	"testing"
)

// Fixture identities the reconcile matrix plants: only runID and owner vary
// across cases; the pids/pgids and their start-identities are fixed so the
// LivenessProbe maps can reference the exact stored values.
const (
	reconcileOwnerPID  = 100
	reconcileChildPGID = 200
	reconcileOwnerID   = "owner-id"
	reconcileChildID   = "child-id"
)

func insertReconcileRun(t *testing.T, store Store, runID, owner string) {
	t.Helper()

	_ = store.Exec(
		`INSERT OR IGNORE INTO agents(owner_id, pid, start_identity, created_at, last_local_seen) VALUES(?,?,?,?,?)`,
		owner, reconcileOwnerPID, reconcileOwnerID, 1, 1)

	err := store.Exec(
		`INSERT INTO runs
			(id, owner_id, command, fix, state, client_token, request_hash,
			 next_seq, child_pgid, child_start_identity, created_at, updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		runID, owner, cmdUsRefine, 1, "running", "tok-"+runID, "h-"+runID, 1,
		reconcileChildPGID, reconcileChildID, 1, 1,
	)
	if err != nil {
		t.Fatalf("insert reconcile run %s: %v", runID, err)
	}
}

// liveSet builds a LivenessProbe that is true ONLY for exact (pid, identity)
// pairs — the identity check that defeats PID/PGID reuse.
func liveSet(pairs map[int]string) LivenessProbe {
	return func(pid int, identity string) bool {
		want, ok := pairs[pid]

		return ok && want == identity
	}
}

func contains(list []string, want string) bool {
	for _, runID := range list {
		if runID == want {
			return true
		}
	}

	return false
}

// signalledOrphan reports whether the verified-orphan child group was signalled;
// the reconcile fixtures only ever plant child group reconcileChildPGID.
func signalledOrphan(list []int) bool {
	for _, pgid := range list {
		if pgid == reconcileChildPGID {
			return true
		}
	}

	return false
}

func TestReconcileDeadOwnerDeadChildAbandons(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	insertReconcileRun(t, store, "run-dead", "owner-dead")

	// Nothing is alive.
	res := store.Reconcile(10, liveSet(map[int]string{}))
	if !contains(res.Abandoned, "run-dead") {
		t.Fatalf("both-dead run must be abandoned: %+v", res)
	}

	if signalledOrphan(res.Terminated) {
		t.Fatalf("a dead child must not be signalled: %+v", res)
	}
}

func TestReconcileDeadOwnerLiveOrphanTerminatesThenAbandons(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	insertReconcileRun(t, store, "run-orphan", "owner-dead")

	// Owner dead, child group ALIVE with the recorded identity.
	res := store.Reconcile(10, liveSet(map[int]string{reconcileChildPGID: reconcileChildID}))
	if !signalledOrphan(res.Terminated) {
		t.Fatalf("a verified live orphan group must be terminated: %+v", res)
	}

	if !contains(res.Abandoned, "run-orphan") {
		t.Fatalf("after terminating the orphan, the run must be abandoned: %+v", res)
	}
}

func TestReconcileLiveSiblingUntouched(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	insertReconcileRun(t, store, "run-live", "owner-live")

	// The owner is alive with its recorded identity.
	res := store.Reconcile(10, liveSet(map[int]string{reconcileOwnerPID: reconcileOwnerID}))
	if contains(res.Abandoned, "run-live") {
		t.Fatalf("a live sibling's run must NEVER be abandoned: %+v", res)
	}

	if !contains(res.Skipped, "run-live") {
		t.Fatalf("a live sibling's run must be skipped: %+v", res)
	}
}

func TestReconcilePidReuseTreatedDead(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	insertReconcileRun(t, store, "run-reuse", "owner-reuse")

	// A DIFFERENT process now occupies pid 100 (identity "recycled"): liveness
	// must never be inferred from the PID alone, so the owner is treated dead.
	res := store.Reconcile(10, liveSet(map[int]string{reconcileOwnerPID: "recycled"}))
	if !contains(res.Abandoned, "run-reuse") {
		t.Fatalf("PID reuse (identity mismatch) must be treated as dead → abandoned: %+v", res)
	}
}

func TestReconcilePgidReuseAbsentLeaderNoSignal(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	insertReconcileRun(t, store, "run-pgid", "owner-dead")

	// pgid 200 is occupied by an UNRELATED group (identity "recycled") — the
	// supervisor identity mismatches, so the group is NOT signalled.
	res := store.Reconcile(10, liveSet(map[int]string{reconcileChildPGID: "recycled"}))
	if signalledOrphan(res.Terminated) {
		t.Fatalf("a mis-identified group (PGID reuse) must not be signalled: %+v", res)
	}

	if !contains(res.Abandoned, "run-pgid") {
		t.Fatalf("owner dead + no verifiable child ⇒ abandoned without a signal: %+v", res)
	}
}

func TestReconcileLeaseSingleWinnerAndFencedTakeover(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	insertReconcileRun(t, store, "run-x", "owner-A")

	const ttl = 1000

	tokenA, okA := store.AcquireReconcileLease("owner-A", 10, ttl)
	if !okA {
		t.Fatal("first claimant must win the reconcile lease")
	}

	if _, okB := store.AcquireReconcileLease("owner-B", 20, ttl); okB {
		t.Fatal("a second claimant must NOT win while the lease is held (single-winner)")
	}

	// After expiry, owner-B takes over with a NEW fencing token.
	tokenB, okB := store.AcquireReconcileLease("owner-B", 10+ttl+1, ttl)
	if !okB {
		t.Fatal("after expiry a new claimant must take over")
	}

	if tokenB == tokenA {
		t.Fatal("takeover must mint a new fencing token")
	}

	// The paused previous claimant's later action (stale token) is rejected.
	err := store.AbandonRunFenced("run-x", "owner-A", tokenA)
	if err == nil {
		t.Fatal("a fenced action with the superseded token must be rejected")
	}

	// The current holder's fenced action succeeds.
	err = store.AbandonRunFenced("run-x", "owner-B", tokenB)
	if err != nil {
		t.Fatalf("the current lease holder's fenced action must succeed: %v", err)
	}
}
