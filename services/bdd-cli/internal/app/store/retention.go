package store

import (
	"context"
	"database/sql"
	"encoding/json"
)

// outputHalves splits the byte budget into a retained head and a retained tail.
const outputHalves = 2

// outputRow is one pruning candidate: an output event's seq and byte weight.
type outputRow struct {
	seq   int
	bytes int
}

// gapPlan describes the middle output range collapsed into one gap.
type gapPlan struct {
	seqs         []int
	fromSeq      int
	throughSeq   int
	droppedBytes int
}

// EnforceByteBudget applies the per-run 256 KiB logical output cap with
// head+tail retention and ONE extendable gap event (plan §1.3). Control events
// are retained independently of output pruning.
func (db *DB) EnforceByteBudget(runID string) {
	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	outputs, total := db.collectOutputs(runID)
	if total <= byteBudget {
		return
	}

	plan, ok := planGap(outputs)
	if !ok {
		return
	}

	db.applyGap(runID, plan)
}

// collectOutputs reads the run's output events (seq, byte weight) in order and
// the running total.
func (db *DB) collectOutputs(runID string) ([]outputRow, int) {
	ctx := context.Background()

	rows, err := db.sql.QueryContext(ctx, `SELECT seq, type, payload FROM events WHERE run_id = ? ORDER BY seq`, runID)
	if err != nil {
		return nil, 0
	}
	defer func() { _ = rows.Close() }()

	var (
		outputs []outputRow
		total   int
	)

	for rows.Next() {
		var (
			seq          int
			typ, payload string
		)

		scanErr := rows.Scan(&seq, &typ, &payload)
		if scanErr != nil {
			return outputs, total
		}

		if typ != evtOutput {
			continue
		}

		var body eventPayload

		_ = json.Unmarshal([]byte(payload), &body)

		outputs = append(outputs, outputRow{seq: seq, bytes: len(body.Data)})
		total += len(body.Data)
	}

	err = rows.Err()
	if err != nil {
		return outputs, total
	}

	return outputs, total
}

// planGap keeps a head half and a tail half of the output, collapsing the
// middle into one gap. ok=false when the middle is empty.
func planGap(outputs []outputRow) (gapPlan, bool) {
	headBudget := byteBudget / outputHalves
	tailBudget := byteBudget - headBudget

	headEnd, acc := 0, 0
	for headEnd < len(outputs) && acc+outputs[headEnd].bytes <= headBudget {
		acc += outputs[headEnd].bytes
		headEnd++
	}

	tailStart, acc := len(outputs), 0
	for tailStart > headEnd && acc+outputs[tailStart-1].bytes <= tailBudget {
		acc += outputs[tailStart-1].bytes
		tailStart--
	}

	if tailStart <= headEnd {
		return gapPlan{}, false
	}

	dropped := outputs[headEnd:tailStart]
	plan := gapPlan{
		seqs:       make([]int, 0, len(dropped)),
		fromSeq:    dropped[0].seq,
		throughSeq: dropped[len(dropped)-1].seq,
	}

	for _, d := range dropped {
		plan.seqs = append(plan.seqs, d.seq)
		plan.droppedBytes += d.bytes
	}

	return plan, true
}

// applyGap deletes the dropped output rows and records exactly ONE gap event,
// merging into an existing gap when present.
func (db *DB) applyGap(runID string, plan gapPlan) {
	ctx := context.Background()

	transaction, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer func() { _ = transaction.Rollback() }()

	for _, seq := range plan.seqs {
		_, err = transaction.ExecContext(ctx, `DELETE FROM events WHERE run_id = ? AND seq = ?`, runID, seq)
		if err != nil {
			return
		}
	}

	if !mergeGap(transaction, runID, plan) {
		body, marshalErr := json.Marshal(eventPayload{
			FromSeq: plan.fromSeq, ThroughSeq: plan.throughSeq, DroppedBytes: plan.droppedBytes,
		})
		if marshalErr != nil {
			return
		}

		_, execErr := transaction.ExecContext(ctx,
			`INSERT INTO events(run_id, seq, type, payload) VALUES(?,?,?,?)`,
			runID, plan.fromSeq, evtGap, string(body),
		)
		if execErr != nil {
			return
		}
	}

	_ = transaction.Commit()
}

// mergeGap extends the run's existing gap event to cover a new dropped range,
// keeping exactly ONE gap. Returns false when no gap exists.
func mergeGap(transaction *sql.Tx, runID string, plan gapPlan) bool {
	ctx := context.Background()

	var (
		seq     int
		payload string
	)

	err := transaction.QueryRowContext(ctx,
		`SELECT seq, payload FROM events WHERE run_id = ? AND type = 'gap' ORDER BY seq LIMIT 1`, runID,
	).Scan(&seq, &payload)
	if err != nil {
		return false
	}

	var body eventPayload

	_ = json.Unmarshal([]byte(payload), &body)

	if body.FromSeq == 0 || plan.fromSeq < body.FromSeq {
		body.FromSeq = plan.fromSeq
	}

	if plan.throughSeq > body.ThroughSeq {
		body.ThroughSeq = plan.throughSeq
	}

	body.DroppedBytes += plan.droppedBytes

	merged, err := json.Marshal(body)
	if err != nil {
		return true
	}

	_, _ = transaction.ExecContext(ctx,
		`UPDATE events SET payload = ? WHERE run_id = ? AND seq = ?`,
		string(merged), runID, seq,
	)

	return true
}

// EnforceRetention prunes terminal runs to the latest terminalRetention per
// project; nonterminal runs are NEVER pruned (plan §1.3). Event/prompt rows
// cascade with the run; dispatch receipts are independent and survive.
func (db *DB) EnforceRetention() {
	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	ctx := context.Background()

	_, _ = db.sql.ExecContext(ctx,
		`DELETE FROM runs
		 WHERE state = 'terminal'
		   AND rowid NOT IN (
		     SELECT rowid FROM runs WHERE state = 'terminal'
		     ORDER BY created_at DESC, rowid DESC LIMIT ?
		   )`,
		terminalRetention,
	)
}

// TerminalRunCount reports how many terminal runs remain (retention gauge).
func (db *DB) TerminalRunCount() int {
	ctx := context.Background()

	var count int

	_ = db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM runs WHERE state = 'terminal'`).Scan(&count)

	return count
}
