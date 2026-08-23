package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// errStaleFencingToken rejects a fenced action carrying a superseded token.
var errStaleFencingToken = errors.New("fenced action rejected: stale fencing token")

// NonterminalRuns returns every run still in flight, joined with its owner's
// pid+identity — the boot-reconciliation working set (plan §1.6).
func (db *DB) NonterminalRuns() []PendingRun {
	ctx := context.Background()

	rows, err := db.sql.QueryContext(ctx,
		`SELECT r.id, r.owner_id, a.pid, a.start_identity,
		        COALESCE(r.child_pgid, 0), COALESCE(r.child_start_identity, '')
		 FROM runs r JOIN agents a ON a.owner_id = r.owner_id
		 WHERE r.state != 'terminal'`,
	)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var out []PendingRun

	for rows.Next() {
		var run PendingRun

		err = rows.Scan(&run.RunID, &run.OwnerID, &run.OwnerPID, &run.OwnerIdentity, &run.ChildPGID, &run.ChildIdentity)
		if err != nil {
			return out
		}

		out = append(out, run)
	}

	err = rows.Err()
	if err != nil {
		return out
	}

	return out
}

// Reconcile is a NON-MUTATING classification pass — it writes nothing so the
// caller can execute the plan FENCED: kill the group, THEN AbandonRunFenced.
// See TestReconcileInspectionIsNonMutating for the reproduced crash window (finding 5).
func (db *DB) Reconcile(_ int64, probe func(pid int, startIdentity string) bool) Reconciled {
	res := Reconciled{}

	for _, run := range db.NonterminalRuns() {
		if probe(run.OwnerPID, run.OwnerIdentity) {
			// Live sibling — never touched.
			res.Skipped = append(res.Skipped, run.RunID)

			continue
		}

		// Owner dead: a verified live orphan group is planned for termination.
		if run.ChildPGID > 0 && probe(run.ChildPGID, run.ChildIdentity) {
			res.Terminated = append(res.Terminated, run.ChildPGID)
		}

		res.Abandoned = append(res.Abandoned, run.RunID)
	}

	return res
}

// AcquireReconcileLease acquires (or takes over an expired) durable single-row
// reconciliation lease, minting a fresh monotonic fencing token (plan §1.6).
// Returns ok=false while a live lease is held by anyone.
func (db *DB) AcquireReconcileLease(ownerID string, nowMs, ttlMs int64) (int64, bool) {
	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	ctx := context.Background()

	transaction, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return 0, false
	}
	defer func() { _ = transaction.Rollback() }()

	var currentToken, expiresAt int64

	err = transaction.QueryRowContext(ctx,
		`SELECT fencing_token, expires_at FROM reconcile_lease WHERE id = 1`,
	).Scan(&currentToken, &expiresAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		token := int64(1)

		_, insErr := transaction.ExecContext(ctx,
			`INSERT INTO reconcile_lease(id, holder_owner_id, fencing_token, acquired_at, expires_at) VALUES(1,?,?,?,?)`,
			ownerID, token, nowMs, nowMs+ttlMs,
		)
		if insErr != nil {
			return 0, false
		}

		if transaction.Commit() != nil {
			return 0, false
		}

		return token, true
	case err != nil:
		return 0, false
	}

	if nowMs < expiresAt {
		// A live lease is held — single-winner.
		return 0, false
	}

	token := currentToken + 1

	_, err = transaction.ExecContext(ctx,
		`UPDATE reconcile_lease SET holder_owner_id = ?, fencing_token = ?, acquired_at = ?, expires_at = ? WHERE id = 1`,
		ownerID, token, nowMs, nowMs+ttlMs,
	)
	if err != nil {
		return 0, false
	}

	if transaction.Commit() != nil {
		return 0, false
	}

	return token, true
}

// RenewReconcileLease extends the lease iff the caller holds the current
// token; it keeps expires_at-acquired_at (the TTL) invariant via SQL delta
// rather than a passed ttl, so repeated renews never drift the TTL (finding 5).
func (db *DB) RenewReconcileLease(ownerID string, token, nowMs int64) bool {
	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	ctx := context.Background()

	res, err := db.sql.ExecContext(ctx,
		`UPDATE reconcile_lease
		 SET expires_at = ? + (expires_at - acquired_at), acquired_at = ?
		 WHERE id = 1 AND holder_owner_id = ? AND fencing_token = ?`,
		nowMs, nowMs, ownerID, token,
	)
	if err != nil {
		return false
	}

	affected, _ := res.RowsAffected()

	return affected > 0
}

// AbandonRunFenced marks a run abandoned only if the caller still holds the
// lease with the current fencing token; a superseded token is rejected (plan
// §1.6 — the paused claimant's later actions never land).
func (db *DB) AbandonRunFenced(runID, ownerID string, token int64) error {
	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	ctx := context.Background()

	transaction, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("fenced abandon: begin: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	var (
		holder  string
		current int64
	)

	err = transaction.QueryRowContext(ctx,
		`SELECT holder_owner_id, fencing_token FROM reconcile_lease WHERE id = 1`,
	).Scan(&holder, &current)
	if err != nil {
		return fmt.Errorf("fenced abandon: no lease: %w", err)
	}

	if holder != ownerID || current != token {
		return fmt.Errorf("%w: token %d (current %d, holder %q)", errStaleFencingToken, token, current, holder)
	}

	_, err = transaction.ExecContext(ctx,
		`UPDATE runs SET state = ?, classification = ?, updated_at = ? WHERE id = ?`,
		stateTerminal, classificationAbandoned, now(), runID,
	)
	if err != nil {
		return fmt.Errorf("fenced abandon: update run: %w", err)
	}

	err = transaction.Commit()
	if err != nil {
		return fmt.Errorf("fenced abandon: commit: %w", err)
	}

	return nil
}
