package store

// The target-schema INTERFACE SEAM the v2 CLI store satisfies (plan §1). Every
// test in this package drives the store exclusively through this seam. The
// production constructors (Open / OpenLocks) now exist, so requireStore /
// requireLocks call them directly and adapt the production DTOs to the seam's
// field-identical test types via the seamStore / seamLocks bridge
// (bridge_internal_test.go).

import (
	"path/filepath"
	"testing"
)

// Shared literals reused across the store package's internal tests, named so
// repeated occurrences do not trip goconst.
const (
	testOwner1  = "owner-1"
	testOwner2  = "owner-2"
	answerApply = "apply"
	answerExit  = "exit"
)

// ── Dispatch (critique §6: the v1 server transaction MOVES INTO the CLI) ──

type DispatchRequest struct {
	OwnerID     string
	Command     string
	StoryID     string
	Fix         bool
	ClientToken string
	RequestHash string
}

// DispatchResult.Kind ∈ created | deduped | conflict | invalid | not_found.
// Pruned is true for an exact retry whose run row was pruned but whose
// dispatch receipt was retained (plan §1.1 — the run read then 404s
// run_pruned).
type DispatchResult struct {
	Kind   string
	RunID  string
	Pruned bool
}

// ── Events + writer actor (critique §9) ──

// Control events (prompt, lock_acquired, terminal) commit synchronously and
// are never dropped; output may be retention-pruned but never unordered.
type Event struct {
	Type         string
	Stream       string
	Data         string
	PromptID     string
	Kind         string
	Payload      string
	Outcome      string
	ErrorDetail  string
	ThroughSeq   int
	DroppedBytes int
	Control      bool
}

type AppendResult struct {
	Seq      int
	Rejected bool
	Terminal bool
}

type RunView struct {
	RunID       string
	OwnerID     string
	State       string
	Outcome     string
	ErrorDetail string
	Answerable  bool
	Pruned      bool
}

// ── Answers (critique §6) ──

// AnswerResult ∈ accepted | conflict | invalid | not_found | cross_owner.
type AnswerResult string

const (
	answerAccepted   AnswerResult = "accepted"
	answerConflict   AnswerResult = "conflict"
	answerInvalid    AnswerResult = "invalid"
	answerNotFound   AnswerResult = "not_found"
	answerCrossOwner AnswerResult = "cross_owner"
)

type AnswerRequest struct {
	OwnerID  string
	RunID    string
	PromptID string
	Kind     string
	Value    string
}

// AnswerDelivery is the seam view of the atomic answer outcome (finding 3): the
// Outcome PLUS whether this submission is the FIRST accept that must be
// delivered (ShouldDeliver — false for an exact retry, so a lost-response retry
// never re-writes stdin) and the prompt's StoredKind.
type AnswerDelivery struct {
	Outcome       AnswerResult
	ShouldDeliver bool
	StoredKind    string
}

// PromptLifecycle exposes the answer/delivery timestamps so the at-most-once
// ordering (delivery_started_at committed BEFORE the stdin write) is
// externally assertable (plan §1.4).
type PromptLifecycle struct {
	AcceptedAt        *int64
	DeliveryStartedAt *int64
	ConsumedAt        *int64
	Answer            string
	AnswerSeq         int
}

// ── Boot reconciliation + fenced lease (plan §1.6) ──

// LivenessProbe lets a test control which owner/child processes are "alive"
// so reconciliation is deterministic (never inferring liveness from a PID).
type LivenessProbe func(pid int, startIdentity string) bool

type NonterminalRun struct {
	RunID         string
	OwnerID       string
	OwnerPID      int
	OwnerIdentity string
	ChildPGID     int
	ChildIdentity string
}

type ReconcileResult struct {
	Abandoned  []string // run ids flipped terminal `abandoned`
	Terminated []int    // pgids signalled (verified orphan groups)
	Skipped    []string // live-sibling runs left untouched
}

// The store seam is split into role interfaces so no single interface exceeds
// the method budget (interfacebloat); Store composes them and the whole set is
// what the v2 CLI store implementation satisfies.

type sqlSeam interface {
	// Low-level (executable-DDL constraint tests via two connections).
	Exec(query string, args ...any) error
	Scalar(query string, args ...any) (int64, error)
}

type dispatchSeam interface {
	Dispatch(req DispatchRequest) DispatchResult
}

type writerSeam interface {
	AppendEvent(runID string, ev Event) AppendResult
	Events(runID string) []Event
	RunView(runID string) (RunView, bool)
}

type answerSeam interface {
	SubmitAnswer(req AnswerRequest) AnswerResult
	ResolveAnswer(req AnswerRequest) AnswerDelivery
	BeginDelivery(runID, promptID string) error
	ConfirmConsumed(runID, promptID string) error
	RecordDeliveryError(runID, promptID, detail string) error
	PromptLifecycle(runID, promptID string) (PromptLifecycle, bool)
}

type retentionSeam interface {
	EnforceByteBudget(runID string)
	EnforceRetention()
	TerminalRunCount() int
}

type reconcileSeam interface {
	NonterminalRuns() []NonterminalRun
	Reconcile(now int64, probe LivenessProbe) ReconcileResult
	AcquireReconcileLease(ownerID string, now, ttlMs int64) (int64, bool)
	RenewReconcileLease(ownerID string, token, now int64) bool
	AbandonRunFenced(runID, ownerID string, token int64) error
}

// Store is the seam the v2 CLI store implementation satisfies.
type Store interface {
	sqlSeam
	dispatchSeam
	writerSeam
	answerSeam
	retentionSeam
	reconcileSeam

	Close() error
}

func requireStore(t *testing.T, path string) Store {
	t.Helper()

	database, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	seam := seamStore{db: database}

	t.Cleanup(func() { _ = seam.Close() })

	return seam
}

func tempDBPath(t *testing.T) string {
	t.Helper()

	return filepath.Join(t.TempDir(), "tmp", "true-bdd-state.db")
}

// ── Scan-vs-command lock protocol seam (plan §1.5, linearizable) ──

// ScanOutcome ∈ ok | inventory_busy. CommandOutcome ∈ ok | folder_locked |
// timeout.
type (
	ScanOutcome    string
	CommandOutcome string
)

const (
	scanOK            ScanOutcome    = "ok"
	scanInventoryBusy ScanOutcome    = "inventory_busy"
	commandOK         CommandOutcome = "ok"
	commandFolderLock CommandOutcome = "folder_locked"
	commandTimeout    CommandOutcome = "timeout"
)

type Lease interface{ Release() }

// LockManager models the two host-tmp lock files (scan.lock,
// command-intent.lock) plus the folder flock, with the exact acquisition
// order of plan §1.5.
type LockManager interface {
	BeginScan() (Lease, ScanOutcome)
	BeginCommand() (Lease, CommandOutcome)
}

func requireLocks(t *testing.T, dir string) LockManager {
	t.Helper()

	locks, err := OpenLocks(dir)
	if err != nil {
		t.Fatalf("open locks: %v", err)
	}

	return seamLocks{locks: locks}
}
