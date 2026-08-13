package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// Dispatch is the v1 server transaction MOVED INTO the CLI store (plan §1.4):
// validate command + bounded token → (owner_id, client_token) receipt lookup →
// request-hash compare (exact retry ⇒ original run; token reuse with different
// args ⇒ conflict) → partial-unique nonterminal-per-owner admission → insert
// run + receipt atomically → reply after commit.
func (db *DB) Dispatch(req DispatchInput) DispatchOutcome {
	if !validCommand(req.Command) {
		return DispatchOutcome{Kind: dispatchInvalid}
	}

	if req.ClientToken == "" || len(req.ClientToken) > tokenMaxBytes {
		return DispatchOutcome{Kind: dispatchInvalid}
	}

	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	ctx := context.Background()

	transaction, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return DispatchOutcome{Kind: dispatchInvalid}
	}
	defer func() { _ = transaction.Rollback() }()

	if out, handled := dispatchByReceipt(transaction, req); handled {
		return out
	}

	out := admitAndInsert(transaction, req)
	if out.Kind == dispatchCreated {
		db.trimReceipts(req.OwnerID)
	}

	return out
}

// dispatchByReceipt resolves an existing (owner, token) receipt: an exact retry
// returns the original run (deduped, possibly pruned); a hash mismatch is a
// conflict. handled=false means no receipt exists (a fresh dispatch).
func dispatchByReceipt(transaction *sql.Tx, req DispatchInput) (DispatchOutcome, bool) {
	ctx := context.Background()

	var run, hash string

	err := transaction.QueryRowContext(ctx,
		`SELECT run_id, request_hash FROM dispatch_receipts WHERE owner_id = ? AND client_token = ?`,
		req.OwnerID, req.ClientToken,
	).Scan(&run, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return DispatchOutcome{}, false
	}

	if err != nil {
		return DispatchOutcome{Kind: dispatchInvalid}, true
	}

	if hash != req.RequestHash {
		return DispatchOutcome{Kind: dispatchConflict}, true
	}

	pruned := !runExists(transaction, run)
	if transaction.Commit() != nil {
		return DispatchOutcome{Kind: dispatchInvalid}, true
	}

	return DispatchOutcome{Kind: dispatchDeduped, RunID: run, Pruned: pruned}, true
}

// admitAndInsert enforces one-nonterminal-run-per-owner, then inserts the run +
// receipt atomically.
func admitAndInsert(transaction *sql.Tx, req DispatchInput) DispatchOutcome {
	ctx := context.Background()

	var active int

	err := transaction.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM runs WHERE owner_id = ? AND state != 'terminal'`, req.OwnerID,
	).Scan(&active)
	if err != nil {
		return DispatchOutcome{Kind: dispatchInvalid}
	}

	if active > 0 {
		return DispatchOutcome{Kind: dispatchConflict}
	}

	runID := newID()
	timestamp := now()

	_, err = transaction.ExecContext(ctx,
		`INSERT INTO runs(
			id, owner_id, command, story_id, fix, state,
			client_token, request_hash, next_seq, created_at, updated_at
		)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		runID, req.OwnerID, req.Command, nullIfEmpty(req.StoryID), boolInt(req.Fix), stateQueued,
		req.ClientToken, req.RequestHash, 1, timestamp, timestamp,
	)
	if err != nil {
		return DispatchOutcome{Kind: dispatchConflict}
	}

	_, err = transaction.ExecContext(ctx,
		`INSERT INTO dispatch_receipts(owner_id, client_token, request_hash, run_id, created_at) VALUES(?,?,?,?,?)`,
		req.OwnerID, req.ClientToken, req.RequestHash, runID, timestamp,
	)
	if err != nil {
		return DispatchOutcome{Kind: dispatchConflict}
	}

	if transaction.Commit() != nil {
		return DispatchOutcome{Kind: dispatchInvalid}
	}

	return DispatchOutcome{Kind: dispatchCreated, RunID: runID}
}

func validCommand(cmd string) bool {
	if strings.TrimSpace(cmd) == "" {
		return false
	}

	if strings.ContainsAny(cmd, " \t\n;&|<>$`\\\"'") {
		return false
	}

	return allowedCommands[cmd]
}

func runExists(transaction *sql.Tx, runID string) bool {
	ctx := context.Background()

	var one int

	err := transaction.QueryRowContext(ctx, `SELECT 1 FROM runs WHERE id = ?`, runID).Scan(&one)

	return err == nil
}

// trimReceipts keeps only the latest receiptRetention receipts for one owner.
func (db *DB) trimReceipts(ownerID string) {
	ctx := context.Background()

	_, _ = db.sql.ExecContext(ctx,
		`DELETE FROM dispatch_receipts
		 WHERE owner_id = ?
		   AND rowid NOT IN (
		     SELECT rowid FROM dispatch_receipts WHERE owner_id = ?
		     ORDER BY created_at DESC, rowid DESC LIMIT ?
		   )`,
		ownerID, ownerID, receiptRetention,
	)
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}

	return s
}

func boolInt(b bool) int {
	if b {
		return 1
	}

	return 0
}
