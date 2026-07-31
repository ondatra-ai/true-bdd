package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// SubmitAnswer applies the answer lifecycle and returns only the outcome
// string. Delivery-aware callers use ResolveAnswer, which additionally reports
// whether this submission is the FIRST accept that must be delivered to the
// child's stdin (plan §1.4, finding 3).
func (db *DB) SubmitAnswer(req AnswerInput) AnswerOutcome {
	return db.ResolveAnswer(req).Outcome
}

// ResolveAnswer applies the answer lifecycle ATOMICALLY (plan §1.4, finding 3):
// pending-prompt currentness, first-wins, exact-retry, terminal-race rejection,
// non-empty clarify/freetext validation, and the cross-owner guard (plan §1.1).
// It returns {Outcome, ShouldDeliver, StoredKind}: an exact retry is Accepted
// with ShouldDeliver=false (a lost-response retry NEVER re-writes stdin), while
// the FIRST accept is Accepted with ShouldDeliver=true. Kind/value validation
// happens BEFORE the commit, so an invalid answer is never persisted.
func (db *DB) ResolveAnswer(req AnswerInput) AnswerResolution {
	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	ctx := context.Background()

	transaction, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return AnswerResolution{Outcome: outcomeInvalid}
	}
	defer func() { _ = transaction.Rollback() }()

	var owner, state string

	err = transaction.QueryRowContext(ctx, `SELECT owner_id, state FROM runs WHERE id = ?`, req.RunID).Scan(&owner, &state)
	if errors.Is(err, sql.ErrNoRows) {
		return AnswerResolution{Outcome: outcomeNotFound}
	}

	if err != nil {
		return AnswerResolution{Outcome: outcomeInvalid}
	}

	// Cross-owner guard: only the owning session may answer.
	if owner != req.OwnerID {
		return AnswerResolution{Outcome: outcomeCrossOwner}
	}

	var (
		kind     string
		existing sql.NullString
	)

	err = transaction.QueryRowContext(ctx,
		`SELECT kind, answer FROM prompts WHERE run_id = ? AND prompt_id = ?`, req.RunID, req.PromptID,
	).Scan(&kind, &existing)
	if errors.Is(err, sql.ErrNoRows) {
		return AnswerResolution{Outcome: outcomeInvalid}
	}

	if err != nil {
		return AnswerResolution{Outcome: outcomeInvalid}
	}

	// Already answered: first-wins with exact-retry (no re-delivery).
	if existing.Valid && existing.String != "" {
		return answerRetry(req, kind, existing.String)
	}

	return storeFreshAnswer(transaction, req, kind, state)
}

// answerRetry resolves a submission against an already-answered prompt: an
// exact retry stays accepted (a lost-response retry) but is NEVER re-delivered
// (ShouldDeliver=false); any other value conflicts (first-wins). No commit is
// needed — the transaction is left to roll back unchanged.
func answerRetry(req AnswerInput, kind, storedAnswer string) AnswerResolution {
	if req.Kind == kind && req.Value == storedAnswer {
		return AnswerResolution{Outcome: outcomeAccepted, ShouldDeliver: false, StoredKind: kind}
	}

	return AnswerResolution{Outcome: outcomeConflict, StoredKind: kind}
}

// storeFreshAnswer validates and records the first answer to a pending prompt,
// rejecting a terminal-race. A fresh accept is the ONLY path that returns
// ShouldDeliver=true.
func storeFreshAnswer(transaction *sql.Tx, req AnswerInput, kind, state string) AnswerResolution {
	if !validAnswer(kind, req.Value) {
		return AnswerResolution{Outcome: outcomeInvalid, StoredKind: kind}
	}

	if state == stateTerminal {
		return AnswerResolution{Outcome: outcomeConflict, StoredKind: kind}
	}

	ctx := context.Background()
	seq := nextSeqLocked(transaction, req.RunID)
	timestamp := now()

	_, err := transaction.ExecContext(ctx,
		`UPDATE prompts SET answer = ?, answer_seq = ?, accepted_at = ? WHERE run_id = ? AND prompt_id = ?`,
		req.Value, seq, timestamp, req.RunID, req.PromptID,
	)
	if err != nil {
		return AnswerResolution{Outcome: outcomeInvalid, StoredKind: kind}
	}

	_, err = transaction.ExecContext(ctx,
		`UPDATE runs SET state = ?, updated_at = ? WHERE id = ? AND state != 'terminal'`,
		stateAnswerAccepted, timestamp, req.RunID,
	)
	if err != nil {
		return AnswerResolution{Outcome: outcomeInvalid, StoredKind: kind}
	}

	err = transaction.Commit()
	if err != nil {
		return AnswerResolution{Outcome: outcomeInvalid, StoredKind: kind}
	}

	return AnswerResolution{Outcome: outcomeAccepted, ShouldDeliver: true, StoredKind: kind}
}

// validAnswer enforces non-empty clarify/freetext and single-line clarify.
func validAnswer(kind, value string) bool {
	switch kind {
	case kindClarify:
		return value != "" && !strings.ContainsAny(value, "\n\r")
	case kindFreetext:
		return value != ""
	case kindChoice:
		return value != ""
	default:
		return value != ""
	}
}

// nextSeqLocked allocates and consumes the run's next event sequence within an
// open write transaction (used for answer_seq bookkeeping).
func nextSeqLocked(transaction *sql.Tx, runID string) int {
	ctx := context.Background()

	var seq int

	err := transaction.QueryRowContext(ctx, `SELECT next_seq FROM runs WHERE id = ?`, runID).Scan(&seq)
	if err != nil {
		return 0
	}

	_, _ = transaction.ExecContext(ctx, `UPDATE runs SET next_seq = ? WHERE id = ?`, seq+1, runID)

	return seq
}

// BeginDelivery commits delivery_started_at BEFORE the stdin write, so
// delivery is at-most-once (plan §1.4): a crash after this commit never
// re-delivers the answer to a fresh child.
func (db *DB) BeginDelivery(runID, promptID string) error {
	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	ctx := context.Background()

	res, err := db.sql.ExecContext(ctx,
		`UPDATE prompts SET delivery_started_at = ? WHERE run_id = ? AND prompt_id = ? AND delivery_started_at IS NULL`,
		now(), runID, promptID,
	)
	if err != nil {
		return fmt.Errorf("begin delivery: %w", err)
	}

	if n, _ := res.RowsAffected(); n == 0 {
		// Either already begun (idempotent) or the prompt is unknown.
		var one int

		scanErr := db.sql.QueryRowContext(ctx,
			`SELECT 1 FROM prompts WHERE run_id = ? AND prompt_id = ?`, runID, promptID,
		).Scan(&one)
		if scanErr != nil {
			return errNoRows
		}
	}

	return nil
}

// RecordDeliveryError records that a store-accepted answer could NOT be
// delivered to the child (no live local executor, or a failed stdin write —
// finding 3). The caller terminates/classifies the run so it never hangs
// answered-but-blocked; this column preserves the diagnostic.
func (db *DB) RecordDeliveryError(runID, promptID, detail string) error {
	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	ctx := context.Background()

	_, err := db.sql.ExecContext(ctx,
		`UPDATE prompts SET delivery_error = ? WHERE run_id = ? AND prompt_id = ?`,
		detail, runID, promptID,
	)
	if err != nil {
		return fmt.Errorf("record delivery error: %w", err)
	}

	return nil
}

// ConfirmConsumed records that the child accepted the answer on stdin. It is
// stamped no earlier than delivery_started_at.
func (db *DB) ConfirmConsumed(runID, promptID string) error {
	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	ctx := context.Background()

	var started sql.NullInt64

	err := db.sql.QueryRowContext(ctx,
		`SELECT delivery_started_at FROM prompts WHERE run_id = ? AND prompt_id = ?`, runID, promptID,
	).Scan(&started)
	if errors.Is(err, sql.ErrNoRows) {
		return errNoRows
	}

	if err != nil {
		return fmt.Errorf("confirm consumed: read delivery: %w", err)
	}

	consumed := now()
	if started.Valid && consumed < started.Int64 {
		consumed = started.Int64
	}

	_, err = db.sql.ExecContext(ctx,
		`UPDATE prompts SET consumed_at = ? WHERE run_id = ? AND prompt_id = ?`, consumed, runID, promptID,
	)
	if err != nil {
		return fmt.Errorf("confirm consumed: %w", err)
	}

	return nil
}

// PromptLifecycle exposes the answer/delivery timestamps for external ordering
// assertions.
func (db *DB) PromptLifecycle(runID, promptID string) (PromptTimes, bool) {
	ctx := context.Background()

	var (
		accepted  sql.NullInt64
		delivered sql.NullInt64
		consumed  sql.NullInt64
		answer    sql.NullString
		answerSeq sql.NullInt64
	)

	err := db.sql.QueryRowContext(ctx,
		`SELECT accepted_at, delivery_started_at, consumed_at, answer, answer_seq
		 FROM prompts WHERE run_id = ? AND prompt_id = ?`, runID, promptID,
	).Scan(&accepted, &delivered, &consumed, &answer, &answerSeq)
	if errors.Is(err, sql.ErrNoRows) {
		return PromptTimes{}, false
	}

	if err != nil {
		return PromptTimes{}, false
	}

	return PromptTimes{
		AcceptedAt:        nullableInt(accepted),
		DeliveryStartedAt: nullableInt(delivered),
		ConsumedAt:        nullableInt(consumed),
		Answer:            answer.String,
		AnswerSeq:         int(answerSeq.Int64),
	}, true
}

func nullableInt(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}

	value := n.Int64

	return &value
}
