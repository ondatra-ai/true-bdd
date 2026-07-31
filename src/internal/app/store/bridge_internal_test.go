package store

// Test bridge (plan §1): adapts the production DTOs to the seam's field-identical
// test types (seam_internal_test.go) with single struct conversions, so the
// tests drive the real *DB / *Locks through the seam interfaces.

// seamStore adapts *DB to the seam Store interface.
type seamStore struct{ db *DB }

func (s seamStore) Exec(query string, args ...any) error { return s.db.Exec(query, args...) }

func (s seamStore) Scalar(query string, args ...any) (int64, error) {
	return s.db.Scalar(query, args...)
}

func (s seamStore) Dispatch(req DispatchRequest) DispatchResult {
	return DispatchResult(s.db.Dispatch(DispatchInput(req)))
}

func (s seamStore) AppendEvent(runID string, ev Event) AppendResult {
	return AppendResult(s.db.AppendEvent(runID, EventInput(ev)))
}

func (s seamStore) Events(runID string) []Event {
	prod := s.db.Events(runID)
	out := make([]Event, len(prod))

	for i, e := range prod {
		out[i] = Event(e)
	}

	return out
}

func (s seamStore) RunView(runID string) (RunView, bool) {
	row, ok := s.db.RunView(runID)

	return RunView(row), ok
}

func (s seamStore) SubmitAnswer(req AnswerRequest) AnswerResult {
	return AnswerResult(s.db.SubmitAnswer(AnswerInput(req)))
}

func (s seamStore) ResolveAnswer(req AnswerRequest) AnswerDelivery {
	r := s.db.ResolveAnswer(AnswerInput(req))

	return AnswerDelivery{Outcome: AnswerResult(r.Outcome), ShouldDeliver: r.ShouldDeliver, StoredKind: r.StoredKind}
}

func (s seamStore) BeginDelivery(runID, promptID string) error {
	return s.db.BeginDelivery(runID, promptID)
}

func (s seamStore) RecordDeliveryError(runID, promptID, detail string) error {
	return s.db.RecordDeliveryError(runID, promptID, detail)
}

func (s seamStore) ConfirmConsumed(runID, promptID string) error {
	return s.db.ConfirmConsumed(runID, promptID)
}

func (s seamStore) PromptLifecycle(runID, promptID string) (PromptLifecycle, bool) {
	times, ok := s.db.PromptLifecycle(runID, promptID)

	return PromptLifecycle(times), ok
}

func (s seamStore) EnforceByteBudget(runID string) { s.db.EnforceByteBudget(runID) }

func (s seamStore) EnforceRetention() { s.db.EnforceRetention() }

func (s seamStore) TerminalRunCount() int { return s.db.TerminalRunCount() }

func (s seamStore) NonterminalRuns() []NonterminalRun {
	prod := s.db.NonterminalRuns()
	out := make([]NonterminalRun, len(prod))

	for i, p := range prod {
		out[i] = NonterminalRun(p)
	}

	return out
}

func (s seamStore) Reconcile(nowMs int64, probe LivenessProbe) ReconcileResult {
	return ReconcileResult(s.db.Reconcile(nowMs, probe))
}

func (s seamStore) AcquireReconcileLease(ownerID string, nowMs, ttlMs int64) (int64, bool) {
	return s.db.AcquireReconcileLease(ownerID, nowMs, ttlMs)
}

func (s seamStore) RenewReconcileLease(ownerID string, token, nowMs int64) bool {
	return s.db.RenewReconcileLease(ownerID, token, nowMs)
}

func (s seamStore) AbandonRunFenced(runID, ownerID string, token int64) error {
	return s.db.AbandonRunFenced(runID, ownerID, token)
}

func (s seamStore) Close() error { return s.db.Close() }

// seamLocks adapts *Locks to the seam LockManager interface.
type seamLocks struct{ locks *Locks }

func (s seamLocks) BeginScan() (Lease, ScanOutcome) {
	lease, outcome := s.locks.BeginScan()
	if lease == nil {
		return nil, ScanOutcome(outcome)
	}

	return lease, ScanOutcome(outcome)
}

func (s seamLocks) BeginCommand() (Lease, CommandOutcome) {
	lease, outcome := s.locks.BeginCommand()
	if lease == nil {
		return nil, CommandOutcome(outcome)
	}

	return lease, CommandOutcome(outcome)
}
