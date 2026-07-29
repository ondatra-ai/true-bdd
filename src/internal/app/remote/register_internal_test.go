package remote

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// TestRegisterRetriesWithStableIncarnation proves NICE-5: a dropped register
// response is retried, and every attempt reuses the ONE per-process
// incarnation id, so the server (which dedups by incarnation) re-registers
// into the SAME session rather than orphaning one.
func TestRegisterRetriesWithStableIncarnation(t *testing.T) {
	t.Parallel()

	var (
		guard        sync.Mutex
		attempts     int
		incarnations []string
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/agent/register", func(writer http.ResponseWriter, request *http.Request) {
		var req RegisterRequest

		_ = json.NewDecoder(request.Body).Decode(&req)

		guard.Lock()
		defer guard.Unlock()

		incarnations = append(incarnations, req.IncarnationID)
		attempts++

		// Drop the first response (the server saw it, the reply is lost).
		if attempts == 1 {
			writer.WriteHeader(http.StatusInternalServerError)

			return
		}

		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(RegisterResponse{SessionID: "sess-1", ScanEpoch: 7})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	incarnation, err := randomID()
	if err != nil {
		t.Fatalf("randomID: %v", err)
	}

	agent := &Agent{
		client:      NewServerClient(server.URL),
		incarnation: incarnation,
		folder:      "/host/x",
		rawFolder:   "/host/x",
		version:     "test",
	}

	err = agent.register(context.Background())
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if agent.sessionID != "sess-1" {
		t.Fatalf("session id = %q, want sess-1", agent.sessionID)
	}

	if agent.scanEpoch != 7 {
		t.Fatalf("scan epoch = %d, want 7", agent.scanEpoch)
	}

	guard.Lock()
	defer guard.Unlock()

	if len(incarnations) < 2 {
		t.Fatalf("expected a retry (>=2 attempts), got %d", len(incarnations))
	}

	for i, seen := range incarnations {
		if seen != incarnation {
			t.Fatalf("attempt %d used incarnation %q, want the stable %q", i, seen, incarnation)
		}
	}
}
