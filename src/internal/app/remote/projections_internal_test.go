package remote

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/src/internal/app/inventory"
	"github.com/ondatra-ai/true-bdd/src/internal/app/store"
)

// newProjectionFixture opens a real store + read handle in a tmpdir, inserts an
// agent row, and dispatches one run owned by that session — the shared setup
// for the projection wire-shape tests.
func newProjectionFixture(t *testing.T) (*store.DB, *readHandle, string, string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "true-bdd-state.db")

	database, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	read, err := openReadHandle(dbPath)
	if err != nil {
		t.Fatalf("open read handle: %v", err)
	}

	t.Cleanup(func() { _ = read.Close() })

	session := "sess-proj-1"
	nowMs := time.Now().UnixMilli()

	err = database.Exec(
		`INSERT INTO agents(owner_id, pid, start_identity, created_at, last_local_seen) VALUES(?,?,?,?,?)`,
		session, 4242, "identity-x", nowMs, nowMs,
	)
	if err != nil {
		t.Fatalf("insert agent: %v", err)
	}

	out := database.Dispatch(store.DispatchInput{
		OwnerID: session, Command: "us-apply", StoryID: "99.3", Fix: true,
		ClientToken: "tok-1", RequestHash: requestHash("us-apply", "99.3", true),
	})
	if out.Kind != dispatchKindCreated {
		t.Fatalf("dispatch kind = %q, want created", out.Kind)
	}

	return database, read, session, out.RunID
}

// TestSessionStatusAndRunDetailProjection proves the store → read-handle →
// projection path produces the exact browser-facing shapes (api-client.ts):
// the active run, project history, a pending prompt with answerable=true, then
// a terminal outcome + envelope with answerable=false.
func TestSessionStatusAndRunDetailProjection(t *testing.T) {
	t.Parallel()

	database, read, session, runID := newProjectionFixture(t)

	// Drive the run through a lock_acquired transition, some output, and a
	// prompt — the same events the executor sinks into the store.
	database.AppendEvent(runID, store.EventInput{Type: eventTypeLockAcquired, Control: true})
	database.AppendEvent(runID, store.EventInput{Type: eventTypeOutput, Stream: streamStdout, Data: "working...\n"})
	database.AppendEvent(runID, store.EventInput{
		Type: eventTypePrompt, PromptID: testPromptID, Kind: kindChoice,
		Payload: `{"actions":["apply","refine","exit"]}`, Control: true,
	})

	status := read.sessionStatus(session)
	assertActiveStatus(t, status, session, runID)

	detail, found, pruned := read.runDetail(session, runID)
	if !found || pruned {
		t.Fatalf("run detail found=%v pruned=%v, want found", found, pruned)
	}

	assertPendingDetail(t, detail, session)
	assertEventShape(t, detail.Events)

	// Answer, then terminate: outcome + envelope populate, answerable clears.
	if got := database.SubmitAnswer(store.AnswerInput{
		OwnerID: session, RunID: runID, PromptID: testPromptID, Kind: kindChoice, Value: "apply",
	}); string(got) != answerOutcomeAccepted {
		t.Fatalf("submit answer = %q, want accepted", got)
	}

	database.AppendEvent(runID, store.EventInput{Type: eventTypeTerminal, Outcome: outcomeConverged, Control: true})

	final, found, _ := read.runDetail(session, runID)
	if !found {
		t.Fatal("terminal run still readable")
	}

	assertTerminalDetail(t, final)
}

// assertActiveStatus checks the session-level projection shape: matching ids, a
// single active run + owner, and the active run's own fields (answerable, story).
func assertActiveStatus(t *testing.T, status SessionStatus, session, runID string) {
	t.Helper()

	if status.SessionID != session || status.OwnerID != session {
		t.Fatalf("status ids = {%q,%q}, want %q", status.SessionID, status.OwnerID, session)
	}

	if len(status.Runs) != 1 || len(status.ActiveOwners) != 1 {
		t.Fatalf("runs=%d active_owners=%d, want 1 and 1", len(status.Runs), len(status.ActiveOwners))
	}

	if status.ActiveOwners[0].SessionID == nil || *status.ActiveOwners[0].SessionID != session {
		t.Fatalf("active owner session_id = %v, want %q", status.ActiveOwners[0].SessionID, session)
	}

	assertActiveRun(t, status.ActiveRun, runID)
}

// assertActiveRun checks the active RunSummary: the expected id, answerable with
// a pending prompt, and the seeded story id.
func assertActiveRun(t *testing.T, run *RunSummary, runID string) {
	t.Helper()

	if run == nil || run.RunID != runID {
		t.Fatalf("active_run = %+v, want run %q", run, runID)
	}

	if !run.Answerable {
		t.Fatal("active run with a pending prompt must be answerable")
	}

	if run.StoryID == nil || *run.StoryID != "99.3" {
		t.Fatalf("story_id = %v, want 99.3", run.StoryID)
	}
}

// assertPendingDetail checks the non-terminal run detail: the serving session, a
// pending choice prompt, and a still-nil envelope.
func assertPendingDetail(t *testing.T, detail RunDetail, session string) {
	t.Helper()

	if detail.SessionID != session {
		t.Fatalf("run detail session_id = %q, want %q (serving session)", detail.SessionID, session)
	}

	prompt := detail.PendingPrompt
	if prompt == nil || prompt.PromptID != testPromptID || prompt.Kind != kindChoice {
		t.Fatalf("pending_prompt = %+v, want prompt-1/choice", detail.PendingPrompt)
	}

	if detail.Envelope != nil {
		t.Fatal("envelope must be nil while the run is non-terminal")
	}
}

// assertTerminalDetail checks the terminal run detail: terminal state, converged
// outcome, no longer answerable, and a converged envelope classification.
func assertTerminalDetail(t *testing.T, final RunDetail) {
	t.Helper()

	if final.State != stateTerminalName {
		t.Fatalf("state = %q, want terminal", final.State)
	}

	if final.Outcome == nil || *final.Outcome != outcomeConverged {
		t.Fatalf("outcome = %v, want converged", final.Outcome)
	}

	if final.Answerable {
		t.Fatal("terminal run must not be answerable")
	}

	envelope := final.Envelope
	if envelope == nil || envelope.Classification == nil || *envelope.Classification != outcomeConverged {
		t.Fatalf("envelope classification = %+v, want converged", final.Envelope)
	}
}

// assertEventShape checks each run event carries a numeric seq + string type
// plus its decoded payload fields (the RunEvent contract).
func assertEventShape(t *testing.T, events []map[string]any) {
	t.Helper()

	if len(events) < 3 {
		t.Fatalf("events = %d, want >= 3 (lock_acquired, output, prompt)", len(events))
	}

	for _, event := range events {
		if _, ok := event["seq"]; !ok {
			t.Fatalf("event missing seq: %v", event)
		}

		if _, ok := event["type"].(string); !ok {
			t.Fatalf("event missing string type: %v", event)
		}
	}

	// The output event's decoded payload must surface stream + data.
	var output map[string]any

	for _, event := range events {
		if event["type"] == eventTypeOutput {
			output = event
		}
	}

	if output == nil || output["stream"] != streamStdout {
		t.Fatalf("output event payload not merged: %v", output)
	}
}

// TestRunDetailPrunedVsNotFound proves the two absent-run cases: a run whose
// receipt survives is run_pruned; a never-seen id is not_found.
func TestRunDetailPrunedVsNotFound(t *testing.T) {
	t.Parallel()

	db, read, session, runID := newProjectionFixture(t)

	// Delete the run row directly, leaving the dispatch receipt (a pruned run).
	err := db.Exec(`DELETE FROM runs WHERE id = ?`, runID)
	if err != nil {
		t.Fatalf("delete run: %v", err)
	}

	if _, found, pruned := read.runDetail(session, runID); found || !pruned {
		t.Fatalf("pruned run: found=%v pruned=%v, want pruned", found, pruned)
	}

	if _, found, pruned := read.runDetail(session, "never-existed"); found || pruned {
		t.Fatalf("unknown run: found=%v pruned=%v, want not_found", found, pruned)
	}
}

// TestDispatchAndAnswerReplyMapping pins the wire status codes the relay
// carries through to the browser (plan §1.4).
func TestDispatchAndAnswerReplyMapping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		kind   string
		runID  string
		status int
	}{
		{dispatchKindCreated, "r1", http.StatusCreated},
		{dispatchKindDeduped, "r2", http.StatusOK},
		{dispatchKindConflict, "", http.StatusConflict},
		{"invalid", "", http.StatusBadRequest},
	}

	for _, testCase := range cases {
		got := dispatchReply(store.DispatchOutcome{Kind: testCase.kind, RunID: testCase.runID})
		if got.Status != testCase.status {
			t.Fatalf("dispatch %q → %d, want %d", testCase.kind, got.Status, testCase.status)
		}
	}

	answers := map[string]int{
		answerOutcomeAccepted:   http.StatusOK,
		answerOutcomeConflict:   http.StatusConflict,
		answerOutcomeCrossOwner: http.StatusConflict,
		answerOutcomeNotFound:   http.StatusNotFound,
		"invalid":               http.StatusBadRequest,
	}

	for outcome, want := range answers {
		if got := answerReply(store.AnswerOutcome(outcome)); got.Status != want {
			t.Fatalf("answer %q → %d, want %d", outcome, got.Status, want)
		}
	}
}

// TestInventoryBudgetFitsNegotiatedCap proves the session_detail inventory
// budget is the negotiated inventory cap MINUS the measured reply envelope
// (plan §1.5), so the serialized snapshot fits the negotiated budget.
func TestInventoryBudgetFitsNegotiatedCap(t *testing.T) {
	t.Parallel()

	const capBytes = 200_000

	agent := &Agent{inventoryBudgetBytes: capBytes}

	status := SessionStatus{SessionID: "s", OwnerID: "s", Runs: []RunSummary{}, ActiveOwners: []ActiveOwner{}}
	budget := agent.inventoryBudget(status)

	if budget <= 0 || budget >= capBytes {
		t.Fatalf("inventory budget = %d, want in (0, %d) — envelope subtracted", budget, capBytes)
	}

	// A scan fitted to that budget serializes within the negotiated cap.
	snapshot := inventory.ScanWithBudget(t.TempDir(), budget)

	envelope := replyEnvelope{Status: http.StatusOK, Body: SessionDetail{SessionStatus: status, Inventory: &snapshot}}

	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal reply: %v", err)
	}

	if len(raw) > capBytes {
		t.Fatalf("serialized reply %d bytes exceeds cap %d", len(raw), capBytes)
	}
}
