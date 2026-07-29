package remote

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/src/internal/app/store"
)

// seedControlRun opens a real store in a tmpdir, inserts an agent row, and
// dispatches one "version" run owned by that agent, returning the store, an
// executor bound to the run, and the folder holding both.
func seedControlRun(t *testing.T) (*store.DB, *RunExecutor, string) {
	t.Helper()

	folder := t.TempDir()
	dbPath := filepath.Join(folder, "tmp", "true-bdd-state.db")

	database, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	nowMs := time.Now().UnixMilli()

	execErr := database.Exec(
		`INSERT INTO agents(owner_id, pid, start_identity, created_at, last_local_seen) VALUES(?,?,?,?,?)`,
		"owner-ctrl", 1, "id", nowMs, nowMs,
	)
	if execErr != nil {
		t.Fatalf("insert agent: %v", execErr)
	}

	out := database.Dispatch(store.DispatchInput{
		OwnerID: "owner-ctrl", Command: commandVersion, ClientToken: "t",
		RequestHash: requestHash(commandVersion, "", false),
	})
	if out.Kind != dispatchKindCreated {
		t.Fatalf("seed dispatch = %q", out.Kind)
	}

	executor := &RunExecutor{store: database, run: RunSpec{RunID: out.RunID, Command: commandVersion}, folder: folder}

	return database, executor, folder
}

// TestControlEventNeverDroppedOnTransientFailure is the inverted regression for
// the reproduced control-event drop (finding 7): the tailer used to advance its
// JSONL offset UNCONDITIONALLY, so a prompt whose store append failed was lost
// and the child blocked forever on an invisible prompt. Now the offset advances
// past a control line ONLY after it durably commits; an unpersistable prompt
// fails the run CLOSED and never advances past the lost event.
func TestControlEventNeverDroppedOnTransientFailure(t *testing.T) {
	t.Parallel()

	database, executor, folder := seedControlRun(t)
	runID := executor.run.RunID

	eventsPath := filepath.Join(folder, "events.jsonl")

	first := `{"type":"prompt","prompt_id":"p1","kind":"choice","payload":{"actions":["apply","refine","exit"]}}` + "\n"

	writeErr := os.WriteFile(eventsPath, []byte(first), 0o600)
	if writeErr != nil {
		t.Fatalf("write events: %v", writeErr)
	}

	// Healthy drain: the prompt is persisted and the offset advances past it.
	offset := executor.drainEvents(eventsPath, 0)
	if offset != int64(len(first)) {
		t.Fatalf("healthy drain must advance past the persisted prompt: offset=%d want=%d", offset, len(first))
	}

	pending, scErr := database.Scalar(`SELECT COUNT(*) FROM prompts WHERE run_id = ? AND prompt_id = 'p1'`, runID)
	if scErr != nil || pending != 1 {
		t.Fatalf("prompt p1 must be persisted: count=%d err=%v", pending, scErr)
	}

	// Now force a TRANSIENT persistence failure by closing the DB, then append a
	// second prompt line the tailer will try (and fail) to persist.
	_ = database.Close()

	second := `{"type":"prompt","prompt_id":"p2","kind":"choice","payload":{}}` + "\n"

	handle, openErr := os.OpenFile(eventsPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if openErr != nil {
		t.Fatalf("append events: %v", openErr)
	}

	_, wErr := handle.WriteString(second)
	if wErr != nil {
		t.Fatalf("write second prompt: %v", wErr)
	}

	_ = handle.Close()

	before := offset
	after := executor.drainEvents(eventsPath, before)

	// The reproduced defect: the offset must NOT advance past a lost control
	// event, and the run must fail closed.
	if after != before {
		t.Fatalf("offset advanced past an unpersistable control event: %d -> %d (finding 7 regression)", before, after)
	}

	if !executor.controlFailed.Load() {
		t.Fatal("an unpersistable control event must fail the run closed")
	}
}
