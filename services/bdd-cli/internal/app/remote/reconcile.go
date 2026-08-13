package remote

import (
	"errors"
	"log/slog"
	"syscall"
	"time"
)

const (
	// orphanTermGrace is how long a verified live orphan group gets between
	// SIGTERM and SIGKILL during boot reconciliation (plan §1.6).
	orphanTermGrace = 2 * time.Second
)

// livenessProbe reports whether the process (pid) is alive AND is the SAME
// process that recorded startIdentity (plan §1.6). It is used for the OWNER
// decision, where the CONSERVATIVE direction on an unverifiable identity is to
// assume the owner is ALIVE — so a possibly-live sibling's run is never
// abandoned. (The signal path uses the STRICT supervisorVerified instead.)
func livenessProbe(pid int, startIdentity string) bool {
	if pid <= 0 {
		return false
	}

	err := syscall.Kill(pid, 0)
	if errors.Is(err, syscall.ESRCH) {
		return false // no such process
	}
	// nil (signalable) or EPERM (exists, not ours) ⇒ the process exists.

	if startIdentity == "" {
		return true // nothing recorded to verify — assume alive (do not abandon)
	}

	current := processStartIdentity(pid)
	if current == "" {
		return true // cannot read ps identity — assume alive (do not abandon)
	}

	return current == startIdentity
}

// supervisorVerified is the STRICT probe evaluated immediately before EACH
// signal (finding 5): it NEVER falls back to bare-PID existence. A missing
// recorded identity, an unreadable current identity, or any mismatch ⇒ false
// (unknown ⇒ skip — never signal by PID alone, defeating PGID reuse).
func supervisorVerified(pgid int, startIdentity string) bool {
	if pgid <= 0 || startIdentity == "" {
		return false
	}

	err := syscall.Kill(pgid, 0)
	if errors.Is(err, syscall.ESRCH) {
		return false
	}

	current := processStartIdentity(pgid)
	if current == "" {
		return false // unreadable identity ⇒ unknown ⇒ never signal
	}

	return current == startIdentity
}

// bootReconcile runs the single-winner, FENCED boot reconciliation BEFORE the
// remote registers (plan §1.6, finding 5): it refreshes this agent's row,
// acquires the durable reconcile lease (skipping if a live claimant holds it),
// then executes a NON-mutating inspection FENCED — draining/killing every
// verified live orphan group FIRST, then abandoning each planned run under the
// SAME fencing token. A live sibling's runs are untouched.
func (a *Agent) bootReconcile() {
	nowMs := time.Now().UnixMilli()

	// Insert/refresh THIS agent's row so the reconcile join sees a valid owner
	// for any run we might later create, and updates our local-seen clock.
	err := a.store.Exec(
		`INSERT OR REPLACE INTO agents(owner_id, pid, start_identity, created_at, last_local_seen) VALUES(?,?,?,?,?)`,
		a.sessionID, a.pid, a.startIdentity, nowMs, nowMs,
	)
	if err != nil {
		slog.Warn("remote: refresh agent row failed", "error", err)
	}

	token, won := a.store.AcquireReconcileLease(a.sessionID, nowMs, reconcileLeaseTTLMillis)
	if !won {
		// A live claimant is reconciling; proceed straight to registration.
		return
	}

	abandoned, terminated := a.reconcilePass(token)

	// Release the lease as soon as this pass completes so a LATER replacement
	// remote can reconcile the runs THIS remote may create and then abandon on
	// its own death (plan §1.6). Expiring the row (not deleting it) preserves
	// fencing-token monotonicity.
	a.releaseReconcileLease(token, time.Now().UnixMilli())

	if abandoned > 0 || terminated > 0 {
		slog.Info("remote: boot reconciliation", "abandoned", abandoned, "terminated_groups", terminated)
	}
}

// reconcilePass executes the fenced reconciliation plan: it inspects (NON-
// mutating) via the store's classifier, DRAINS/KILLS every planned orphan group
// FIRST (re-validating the supervisor identity and renewing the token before
// EACH signal), THEN abandons each planned run under the SAME token. Draining
// every group before ANY terminal write closes the crash-between-kill-and-
// abandon window (finding 5). It STOPS the instant the lease is lost.
func (a *Agent) reconcilePass(token int64) (int, int) {
	// NON-mutating inspection. livenessProbe (conservative) protects live
	// siblings; the strict supervisorVerified re-checks before each signal.
	plan := a.store.Reconcile(time.Now().UnixMilli(), livenessProbe)

	identityByPGID := map[int]string{}

	for _, run := range a.store.NonterminalRuns() {
		if run.ChildPGID > 0 {
			identityByPGID[run.ChildPGID] = run.ChildIdentity
		}
	}

	abandoned, terminated := 0, 0

	// DRAIN/KILL every planned orphan group FIRST — before ANY abandon.
	for _, pgid := range plan.Terminated {
		if a.terminateOrphanFenced(pgid, identityByPGID[pgid], token) {
			terminated++
		}
	}

	// THEN abandon each planned run under the CURRENT fencing token. A
	// superseded token (a paused claimant taken over) is rejected by the store.
	for _, runID := range plan.Abandoned {
		if !a.store.RenewReconcileLease(a.sessionID, token, time.Now().UnixMilli()) {
			return abandoned, terminated // lost the lease — stop acting
		}

		err := a.store.AbandonRunFenced(runID, a.sessionID, token)
		if err == nil {
			abandoned++
		}
	}

	return abandoned, terminated
}

// terminateOrphanFenced drains then kills a verified live orphan group,
// re-validating the SUPERVISOR identity AND renewing the fencing lease
// immediately before EACH signal (finding 5). A missing/unreadable identity is
// never signalled. Returns whether the group was signalled.
func (a *Agent) terminateOrphanFenced(pgid int, identity string, token int64) bool {
	if pgid <= 0 {
		return false
	}

	// SIGTERM — verify the supervisor identity + renew the lease right before.
	if !supervisorVerified(pgid, identity) {
		return false
	}

	if !a.store.RenewReconcileLease(a.sessionID, token, time.Now().UnixMilli()) {
		return false
	}

	_ = syscall.Kill(-pgid, syscall.SIGTERM)

	time.Sleep(orphanTermGrace)

	// SIGKILL — re-verify the identity AGAIN immediately before the second
	// signal (the leader could have exited and its pgid been reused).
	if !supervisorVerified(pgid, identity) {
		return true // already gone / no longer ours — the SIGTERM sufficed
	}

	if !a.store.RenewReconcileLease(a.sessionID, token, time.Now().UnixMilli()) {
		return true
	}

	_ = syscall.Kill(-pgid, syscall.SIGKILL)

	return true
}

// releaseReconcileLease expires this remote's boot-reconciliation lease so the
// next replacement remote can take it over immediately (plan §1.6).
func (a *Agent) releaseReconcileLease(token, nowMs int64) {
	err := a.store.Exec(
		`UPDATE reconcile_lease SET expires_at = ? WHERE holder_owner_id = ? AND fencing_token = ?`,
		nowMs, a.sessionID, token,
	)
	if err != nil {
		slog.Warn("remote: release reconcile lease failed", "error", err)
	}
}
