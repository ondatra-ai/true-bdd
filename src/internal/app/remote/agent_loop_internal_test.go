package remote

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/src/internal/app/store"
)

// capturedReply records what the CLI posted to /api/agent/reply.
type capturedReply struct {
	sessionID string
	epoch     string
	token     string
	workID    string
	envelope  replyEnvelope
}

// newLoopAgent builds an Agent wired to a real store + read handle in a
// tmpdir, targeting the given relay URL — bypassing newAgent's os.Getwd().
func newLoopAgent(t *testing.T, serverURL string) (*Agent, string, string) {
	t.Helper()

	folder := t.TempDir()
	dbPath := filepath.Join(folder, "tmp", "true-bdd-state.db")

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

	locks, err := store.OpenLocks(filepath.Join(folder, "tmp"))
	if err != nil {
		t.Fatalf("open locks: %v", err)
	}

	session := "sess-loop-1"
	nowMs := time.Now().UnixMilli()

	execErr := database.Exec(
		`INSERT INTO agents(owner_id, pid, start_identity, created_at, last_local_seen) VALUES(?,?,?,?,?)`,
		session, 1, "id", nowMs, nowMs,
	)
	if execErr != nil {
		t.Fatalf("insert agent: %v", execErr)
	}

	agent := &Agent{
		client:    NewRelayClient(serverURL),
		store:     database,
		locks:     locks,
		read:      read,
		folder:    folder,
		rawFolder: folder,
		sessionID: session,
		pid:       1,
	}

	return agent, session, dbPath
}

// TestRegisterPollReplyLoopDeliversQuery drives the whole v2 protocol path:
// register → poll (one session_status query work item) → the CLI projects the
// store and replies with the correct correlation headers and a 200 body
// carrying the active run.
func TestRegisterPollReplyLoopDeliversQuery(t *testing.T) {
	t.Parallel()

	var (
		registered atomic.Int32
		polls      atomic.Int32
		replies    = make(chan capturedReply, 1)
	)

	agentHolder := make(chan *Agent, 1)

	server := newLoopServer(t, loopServerDeps{
		registered:  &registered,
		polls:       &polls,
		replies:     replies,
		agentHolder: agentHolder,
	})
	defer server.Close()

	agent, session, _ := newLoopAgent(t, server.URL)
	agentHolder <- agent

	// Seed a non-terminal run so the session_status projection is non-empty.
	out := agent.store.Dispatch(store.DispatchInput{
		OwnerID: session, Command: commandVersion, ClientToken: "t",
		RequestHash: requestHash(commandVersion, "", false),
	})
	if out.Kind != dispatchKindCreated {
		t.Fatalf("seed dispatch = %q", out.Kind)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- agent.loop(ctx) }()

	select {
	case reply := <-replies:
		assertQueryReply(t, reply, session, out.RunID)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the CLI reply")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("loop did not return after cancel")
	}

	if registered.Load() == 0 {
		t.Fatal("the CLI never registered")
	}
}

// loopServerDeps bundles the shared state the fake relay's handlers close over.
type loopServerDeps struct {
	registered  *atomic.Int32
	polls       *atomic.Int32
	replies     chan capturedReply
	agentHolder chan *Agent
}

// newLoopServer stands up the fake v2 relay: register hands out a capability
// token, poll delivers exactly ONE session_status query then 204s, and reply
// captures the CLI's correlated response.
func newLoopServer(t *testing.T, deps loopServerDeps) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/agent/register", func(writer http.ResponseWriter, _ *http.Request) {
		deps.registered.Add(1)
		writeJSON(t, writer, registerResult{ConnectionEpoch: 1, ReplyBudgetBytes: 1_048_576, CapabilityToken: capToken})
	})

	mux.HandleFunc("/api/agent/poll", func(writer http.ResponseWriter, request *http.Request) {
		var req pollRequest

		_ = json.NewDecoder(request.Body).Decode(&req)

		if req.CapabilityToken != capToken || req.ConnectionEpoch != 1 {
			t.Errorf("poll correlation = {%q,%d}, want {cap,1}", req.CapabilityToken, req.ConnectionEpoch)
		}

		// Deliver exactly ONE query work item on the first poll; 204 after.
		if deps.polls.Add(1) == 1 {
			agent := <-deps.agentHolder
			deps.agentHolder <- agent

			payload, _ := json.Marshal(queryPayload{View: viewSessionStatus, SessionID: agent.sessionID})
			writeJSON(t, writer, workItem{
				WorkID: "w1", SessionID: agent.sessionID, ConnectionEpoch: 1,
				Type: workQuery, Payload: payload, Deadline: time.Now().Add(30 * time.Second).UnixMilli(),
			})

			return
		}

		time.Sleep(20 * time.Millisecond) // throttle the 204 repoll spin
		writer.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/api/agent/reply", func(writer http.ResponseWriter, request *http.Request) {
		var env replyEnvelope

		_ = json.NewDecoder(request.Body).Decode(&env)

		deps.replies <- capturedReply{
			sessionID: request.Header.Get("X-Session-Id"),
			epoch:     request.Header.Get("X-Connection-Epoch"),
			token:     request.Header.Get("X-Capability-Token"),
			workID:    request.Header.Get("X-Work-Id"),
			envelope:  env,
		}

		writeJSON(t, writer, map[string]any{"ok": true})
	})

	return httptest.NewServer(mux)
}

// assertQueryReply checks the correlation headers and that the body is a 200
// session_status carrying the seeded active run.
func assertQueryReply(t *testing.T, reply capturedReply, session, runID string) {
	t.Helper()

	if reply.workID != "w1" || reply.sessionID != session || reply.epoch != "1" || reply.token != capToken {
		t.Fatalf("reply correlation = {%q,%q,%q,%q}", reply.workID, reply.sessionID, reply.epoch, reply.token)
	}

	if reply.envelope.Status != http.StatusOK {
		t.Fatalf("reply status = %d, want 200", reply.envelope.Status)
	}

	// The body round-trips through JSON exactly as the relay would carry it.
	raw, err := json.Marshal(reply.envelope.Body)
	if err != nil {
		t.Fatalf("marshal reply body: %v", err)
	}

	var status SessionStatus

	err = json.Unmarshal(raw, &status)
	if err != nil {
		t.Fatalf("reply body is not a SessionStatus: %v", err)
	}

	if status.ActiveRun == nil || status.ActiveRun.RunID != runID {
		t.Fatalf("reply active_run = %+v, want run %q", status.ActiveRun, runID)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, body any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(w).Encode(body)
	if err != nil {
		t.Errorf("encode response: %v", err)
	}
}
