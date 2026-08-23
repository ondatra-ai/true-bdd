package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// eventPayload is the on-disk JSON body of an events row. Control and output
// events share one generic ledger (plan §1.1); the fields relevant to a given
// type are populated, the rest omitted.
type eventPayload struct {
	Stream       string `json:"stream,omitempty"`
	Data         string `json:"data,omitempty"`
	PromptID     string `json:"prompt_id,omitempty"`
	Kind         string `json:"kind,omitempty"`
	Payload      string `json:"payload,omitempty"`
	Outcome      string `json:"outcome,omitempty"`
	ErrorDetail  string `json:"error_detail,omitempty"`
	FromSeq      int    `json:"from_seq,omitempty"`
	ThroughSeq   int    `json:"through_seq,omitempty"`
	DroppedBytes int    `json:"dropped_bytes,omitempty"`
}

// controlType reports whether an event type is a control event (prompt,
// lock_acquired, terminal, gap) — never dropped by retention.
func controlType(t string) bool {
	switch t {
	case evtPrompt, evtLockAcquired, evtTerminal, evtGap:
		return true
	default:
		return false
	}
}

// AppendEvent is the writer-actor entry point: it allocates next_seq and
// inserts the event in ONE transaction under db.writeMu (see the DB doc) so
// concurrent producers get unique, contiguous sequences.
func (db *DB) AppendEvent(runID string, event EventInput) AppendOutcome {
	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	ctx := context.Background()

	transaction, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return AppendOutcome{Rejected: true}
	}
	defer func() { _ = transaction.Rollback() }()

	var (
		state   string
		nextSeq int
	)

	err = transaction.QueryRowContext(ctx, `SELECT state, next_seq FROM runs WHERE id = ?`, runID).Scan(&state, &nextSeq)
	if errors.Is(err, sql.ErrNoRows) {
		// The run is gone — a permanent, safe drop (finding 7).
		return AppendOutcome{Rejected: true, Terminal: true}
	}

	if err != nil {
		// Transient read failure — the caller retries / fails closed.
		return AppendOutcome{Rejected: true}
	}

	if state == stateTerminal {
		// The run is finished; nothing may follow the terminal event — a
		// permanent, safe drop (finding 7).
		return AppendOutcome{Rejected: true, Terminal: true}
	}

	seq := nextSeq
	body := eventPayload{
		Stream:       event.Stream,
		Data:         event.Data,
		PromptID:     event.PromptID,
		Kind:         event.Kind,
		Payload:      event.Payload,
		Outcome:      event.Outcome,
		ErrorDetail:  event.ErrorDetail,
		ThroughSeq:   event.ThroughSeq,
		DroppedBytes: event.DroppedBytes,
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return AppendOutcome{Rejected: true}
	}

	_, err = transaction.ExecContext(ctx,
		`INSERT INTO events(run_id, seq, type, payload) VALUES(?,?,?,?)`,
		runID, seq, event.Type, string(raw),
	)
	if err != nil {
		return AppendOutcome{Rejected: true}
	}

	_, err = transaction.ExecContext(ctx, `UPDATE runs SET next_seq = ?, updated_at = ? WHERE id = ?`, seq+1, now(), runID)
	if err != nil {
		return AppendOutcome{Rejected: true}
	}

	err = applyProjection(transaction, runID, seq, event)
	if err != nil {
		return AppendOutcome{Rejected: true}
	}

	err = transaction.Commit()
	if err != nil {
		return AppendOutcome{Rejected: true}
	}

	return AppendOutcome{Seq: seq}
}

// applyProjection folds the event into the run's projected columns (plan
// §1.2): the lock-proof transition, prompt publication, and the terminal
// state/classification.
func applyProjection(transaction *sql.Tx, runID string, seq int, event EventInput) error {
	ctx := context.Background()

	switch event.Type {
	case evtLockAcquired:
		_, err := transaction.ExecContext(ctx,
			`UPDATE runs SET state = 'running' WHERE id = ? AND state != 'terminal'`,
			runID,
		)
		if err != nil {
			return fmt.Errorf("projection lock_acquired: %w", err)
		}

		return nil
	case evtPrompt:
		_, err := transaction.ExecContext(ctx,
			`INSERT INTO prompts(run_id, prompt_id, event_seq, kind, payload) VALUES(?,?,?,?,?)`,
			runID, event.PromptID, seq, event.Kind, nullIfEmpty(event.Payload),
		)
		if err != nil {
			return fmt.Errorf("projection prompt insert: %w", err)
		}

		_, err = transaction.ExecContext(ctx, `UPDATE runs SET state = ? WHERE id = ?`, statePromptPublished, runID)
		if err != nil {
			return fmt.Errorf("projection prompt state: %w", err)
		}

		return nil
	case evtTerminal:
		_, err := transaction.ExecContext(ctx,
			`UPDATE runs SET state = ?, classification = ?, engine_outcome = ?, error_detail = ?, updated_at = ? WHERE id = ?`,
			stateTerminal, nullIfEmpty(event.Outcome), nullIfEmpty(event.Outcome), nullIfEmpty(event.ErrorDetail), now(), runID,
		)
		if err != nil {
			return fmt.Errorf("projection terminal: %w", err)
		}

		return nil
	default:
		return nil
	}
}

// Events returns the run's events in seq order, decoded into the seam shape.
func (db *DB) Events(runID string) []EventInput {
	ctx := context.Background()

	rows, err := db.sql.QueryContext(ctx, `SELECT seq, type, payload FROM events WHERE run_id = ? ORDER BY seq`, runID)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var out []EventInput

	for rows.Next() {
		var (
			seq          int
			typ, payload string
		)

		err = rows.Scan(&seq, &typ, &payload)
		if err != nil {
			return out
		}

		var body eventPayload

		_ = json.Unmarshal([]byte(payload), &body)

		out = append(out, EventInput{
			Type:         typ,
			Stream:       body.Stream,
			Data:         body.Data,
			PromptID:     body.PromptID,
			Kind:         body.Kind,
			Payload:      body.Payload,
			Outcome:      body.Outcome,
			ErrorDetail:  body.ErrorDetail,
			ThroughSeq:   body.ThroughSeq,
			DroppedBytes: body.DroppedBytes,
			Control:      controlType(typ),
		})
	}

	err = rows.Err()
	if err != nil {
		return out
	}

	return out
}

// RunView returns the projected run row, or ok=false when the run is absent.
func (db *DB) RunView(runID string) (RunRow, bool) {
	ctx := context.Background()

	var (
		ownerID        string
		state          string
		classification sql.NullString
		errorDetail    sql.NullString
	)

	err := db.sql.QueryRowContext(ctx,
		`SELECT owner_id, state, classification, error_detail FROM runs WHERE id = ?`, runID,
	).Scan(&ownerID, &state, &classification, &errorDetail)
	if errors.Is(err, sql.ErrNoRows) {
		return RunRow{}, false
	}

	if err != nil {
		return RunRow{}, false
	}

	answerable := false

	if state != stateTerminal {
		var pending int

		_ = db.sql.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM prompts WHERE run_id = ? AND (answer IS NULL OR answer = '')`, runID,
		).Scan(&pending)

		answerable = pending > 0
	}

	return RunRow{
		RunID:       runID,
		OwnerID:     ownerID,
		State:       state,
		Outcome:     classification.String,
		ErrorDetail: errorDetail.String,
		Answerable:  answerable,
	}, true
}
