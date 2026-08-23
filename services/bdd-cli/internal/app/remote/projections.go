package remote

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (same as the store)
)

// readHandle is a SECOND, READ-ONLY *sql.DB on the same per-project SQLite
// file the store owns (plan §3); writes always go through store.DB. Each
// projection runs ONE read tx, no nested query mid-cursor (finding 6).
type readHandle struct {
	sql *sql.DB
}

// openReadHandle opens the read connection to the store's db file in read-only
// mode (mode=ro + query_only), so it can never mutate the store and never
// upgrades a read to a write that could contend with the writer (finding 6).
func openReadHandle(dbPath string) (*readHandle, error) {
	dsn := "file:" + dbPath +
		"?mode=ro" +
		"&_pragma=query_only(1)" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=busy_timeout(10000)"

	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open read handle: %w", err)
	}

	database.SetMaxOpenConns(maxReadConns)

	return &readHandle{sql: database}, nil
}

// maxReadConns caps the read pool. Because each projection now uses exactly ONE
// connection for its whole snapshot (no nested query needing a second), this is
// a throughput knob, not a deadlock hazard (finding 6).
const maxReadConns = 4

// Close closes the read connection.
func (r *readHandle) Close() error {
	err := r.sql.Close()
	if err != nil {
		return fmt.Errorf("close read handle: %w", err)
	}

	return nil
}

// readTx runs body inside ONE read-only transaction — a consistent snapshot on
// a single connection (finding 6). A begin failure leaves body unrun.
func (r *readHandle) readTx(body func(transaction *sql.Tx)) {
	transaction, err := r.sql.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return
	}
	defer func() { _ = transaction.Rollback() }()

	body(transaction)
}

// sessionStatus builds the `session_status` projection (plan §3): the active
// run for this owner, project run history newest-first, and same-project active
// owners — all from ONE consistent snapshot.
func (r *readHandle) sessionStatus(sessionID string) SessionStatus {
	runs := []RunSummary{}

	r.readTx(func(transaction *sql.Tx) {
		runs = allRunsTx(transaction, sessionID)
	})

	var active *RunSummary

	for index := range runs {
		if runs[index].OwnerID == sessionID && runs[index].State != stateTerminalName {
			clone := runs[index]
			active = &clone

			break
		}
	}

	return SessionStatus{
		SessionID:    sessionID,
		OwnerID:      sessionID,
		ActiveRun:    active,
		Runs:         runs,
		ActiveOwners: activeOwnersFrom(runs),
	}
}

// allRunsTx returns every run in the project newest-first with the per-row
// `answerable` fact computed by an EXISTS subquery (NO nested query while the
// cursor is open — the deadlock fix, finding 6).
func allRunsTx(transaction *sql.Tx, sessionID string) []RunSummary {
	rows, err := transaction.QueryContext(context.Background(),
		`SELECT r.id, r.owner_id, r.command, r.story_id, r.fix, r.state, r.classification, r.error_detail,
		        r.created_at, r.updated_at,
		        EXISTS(SELECT 1 FROM prompts p WHERE p.run_id = r.id AND (p.answer IS NULL OR p.answer = '')) AS has_pending
		 FROM runs r ORDER BY r.created_at DESC, r.rowid DESC`,
	)
	if err != nil {
		return []RunSummary{}
	}
	defer func() { _ = rows.Close() }()

	out := []RunSummary{}

	for rows.Next() {
		summary, ok := scanRunSummaryRow(rows, sessionID)
		if !ok {
			return out
		}

		out = append(out, summary)
	}

	if rows.Err() != nil {
		return out
	}

	return out
}

// scanRunSummaryRow scans one run row (including the EXISTS has_pending column)
// into a RunSummary, computing the nullable outcome and the cross-owner
// answerable flag (plan §1.1).
func scanRunSummaryRow(rows *sql.Rows, sessionID string) (RunSummary, bool) {
	var (
		runID, owner, command, state string
		storyID, classification      sql.NullString
		errorDetail                  sql.NullString
		fix, hasPending              int
		createdAt, updatedAt         int64
	)

	err := rows.Scan(&runID, &owner, &command, &storyID, &fix, &state,
		&classification, &errorDetail, &createdAt, &updatedAt, &hasPending)
	if err != nil {
		return RunSummary{}, false
	}

	summary := RunSummary{
		RunID:       runID,
		OwnerID:     owner,
		Command:     command,
		StoryID:     nsPtr(storyID),
		Fix:         fix != 0,
		State:       state,
		ErrorDetail: nonEmptyPtr(errorDetail),
		Answerable:  owner == sessionID && state != stateTerminalName && hasPending != 0,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}

	if state == stateTerminalName {
		summary.Outcome = nsPtr(classification)
	}

	return summary, true
}

// activeOwnersFrom derives the distinct owners with a non-terminal run — the
// sibling-warning working set (plan §3) — from the already-materialized run
// history (no second query; see readHandle, finding 6). owner_id IS the session id in v2.
func activeOwnersFrom(runs []RunSummary) []ActiveOwner {
	seen := map[string]bool{}
	out := []ActiveOwner{}

	for index := range runs {
		if runs[index].State == stateTerminalName || seen[runs[index].OwnerID] {
			continue
		}

		seen[runs[index].OwnerID] = true
		owner := runs[index].OwnerID
		out = append(out, ActiveOwner{
			OwnerID:   owner,
			SessionID: &owner,
			RunID:     runs[index].RunID,
			Command:   runs[index].Command,
		})
	}

	return out
}

// runDetail builds the `run_detail` projection (plan §3) from ONE snapshot.
// found=false with pruned=true means the run row is gone but a dispatch receipt
// remains (404 run_pruned); pruned=false means it never existed (404 not_found).
func (r *readHandle) runDetail(sessionID, runID string) (RunDetail, bool, bool) {
	var (
		detail RunDetail
		found  bool
		pruned bool
	)

	r.readTx(func(transaction *sql.Tx) {
		detail, found, pruned = runDetailTx(transaction, sessionID, runID)
	})

	return detail, found, pruned
}

// runDetailTx projects the run detail within a single read transaction:
// the run row (+ EXISTS has_pending), then the fully-materialized event window,
// then the pending prompt — sequentially on ONE connection (finding 6).
func runDetailTx(transaction *sql.Tx, sessionID, runID string) (RunDetail, bool, bool) {
	var (
		owner, command, state              string
		storyID, classification, errDetail sql.NullString
		engineOutcome, signal              sql.NullString
		finalizationOK, exitCode           sql.NullInt64
		fix, hasPending                    int
		createdAt, updatedAt               int64
	)

	row := transaction.QueryRowContext(context.Background(),
		`SELECT owner_id, command, story_id, fix, state, classification, error_detail,
		        engine_outcome, finalization_ok, exit_code, signal, created_at, updated_at,
		        EXISTS(SELECT 1 FROM prompts p WHERE p.run_id = ? AND (p.answer IS NULL OR p.answer = '')) AS has_pending
		 FROM runs WHERE id = ?`, runID, runID,
	)

	scanErr := row.Scan(&owner, &command, &storyID, &fix, &state, &classification, &errDetail,
		&engineOutcome, &finalizationOK, &exitCode, &signal, &createdAt, &updatedAt, &hasPending)
	if errors.Is(scanErr, sql.ErrNoRows) {
		return RunDetail{}, false, receiptExistsTx(transaction, runID)
	}

	if scanErr != nil {
		return RunDetail{}, false, false
	}

	summary := RunSummary{
		RunID:       runID,
		OwnerID:     owner,
		Command:     command,
		StoryID:     nsPtr(storyID),
		Fix:         fix != 0,
		State:       state,
		ErrorDetail: nonEmptyPtr(errDetail),
		Answerable:  owner == sessionID && state != stateTerminalName && hasPending != 0,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}

	if state == stateTerminalName {
		summary.Outcome = nsPtr(classification)
	}

	// Materialize the event window (close its cursor) BEFORE the pending-prompt
	// query, so both run sequentially on the transaction's single connection.
	detail := RunDetail{
		RunSummary:    summary,
		SessionID:     sessionID,
		Events:        eventsTx(transaction, runID),
		PendingPrompt: pendingPromptTx(transaction, runID, state),
	}

	if state == stateTerminalName {
		detail.Envelope = &TerminalEnvelope{
			Classification: nsPtr(classification),
			EngineOutcome:  nsPtr(engineOutcome),
			FinalizationOK: nbPtr(finalizationOK),
			ExitCode:       niPtr(exitCode),
			Signal:         nsPtr(signal),
		}
	}

	return detail, true, false
}

// eventsTx returns the run's entire current capped event window (plan §3), each
// as {seq, type, ...payloadFields}, fully materialized before returning.
func eventsTx(transaction *sql.Tx, runID string) []map[string]any {
	rows, err := transaction.QueryContext(context.Background(),
		`SELECT seq, type, payload FROM events WHERE run_id = ? ORDER BY seq`, runID)
	if err != nil {
		return []map[string]any{}
	}
	defer func() { _ = rows.Close() }()

	out := []map[string]any{}

	for rows.Next() {
		var (
			seq          int
			typ, payload string
		)

		scanErr := rows.Scan(&seq, &typ, &payload)
		if scanErr != nil {
			return out
		}

		event := map[string]any{"seq": seq, "type": typ}

		fields := map[string]any{}
		if json.Unmarshal([]byte(payload), &fields) == nil {
			for key, value := range fields {
				event[key] = value
			}
		}

		out = append(out, event)
	}

	if rows.Err() != nil {
		return out
	}

	return out
}

// pendingPromptTx returns the current unanswered prompt (plan §3), or nil when
// the run is terminal or fully answered.
func pendingPromptTx(transaction *sql.Tx, runID, state string) *PendingPrompt {
	if state == stateTerminalName {
		return nil
	}

	var (
		promptID, kind string
		payload        sql.NullString
	)

	err := transaction.QueryRowContext(context.Background(),
		`SELECT prompt_id, kind, payload FROM prompts
		 WHERE run_id = ? AND (answer IS NULL OR answer = '')
		 ORDER BY event_seq LIMIT 1`, runID,
	).Scan(&promptID, &kind, &payload)
	if err != nil {
		return nil
	}

	prompt := &PendingPrompt{PromptID: promptID, Kind: kind}

	if payload.Valid && payload.String != "" {
		var decoded any
		if json.Unmarshal([]byte(payload.String), &decoded) == nil {
			prompt.Payload = decoded
		}
	}

	return prompt
}

// promptKind returns the kind of a prompt so the answer handler can validate
// and deliver it (plan §1.4). A single autocommit read — no nested cursor.
func (r *readHandle) promptKind(runID, promptID string) (string, bool) {
	var kind string

	err := r.sql.QueryRowContext(context.Background(),
		`SELECT kind FROM prompts WHERE run_id = ? AND prompt_id = ?`, runID, promptID,
	).Scan(&kind)
	if err != nil {
		return "", false
	}

	return kind, true
}

// runCommand returns a run's command/story/fix so the executor can spawn it.
func (r *readHandle) runCommand(runID string) (RunSpec, bool) {
	var (
		command string
		storyID sql.NullString
		fix     int
	)

	err := r.sql.QueryRowContext(context.Background(),
		`SELECT command, story_id, fix FROM runs WHERE id = ?`, runID).Scan(&command, &storyID, &fix)
	if err != nil {
		return RunSpec{}, false
	}

	return RunSpec{RunID: runID, Command: command, StoryID: storyID.String, Fix: fix != 0}, true
}

// receiptExistsTx reports whether a dispatch receipt still references runID (a
// pruned-run marker, plan §1.1), on the projection's transaction.
func receiptExistsTx(transaction *sql.Tx, runID string) bool {
	var one int

	err := transaction.QueryRowContext(context.Background(),
		`SELECT 1 FROM dispatch_receipts WHERE run_id = ? LIMIT 1`, runID).Scan(&one)

	return err == nil
}

// ── nullable → pointer helpers (JSON null vs value) ──

func nsPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}

	value := ns.String

	return &value
}

// nonEmptyPtr maps an absent OR empty string to nil (a null error_detail).
func nonEmptyPtr(ns sql.NullString) *string {
	if !ns.Valid || ns.String == "" {
		return nil
	}

	value := ns.String

	return &value
}

func niPtr(ni sql.NullInt64) *int {
	if !ni.Valid {
		return nil
	}

	value := int(ni.Int64)

	return &value
}

func nbPtr(ni sql.NullInt64) *bool {
	if !ni.Valid {
		return nil
	}

	value := ni.Int64 != 0

	return &value
}
