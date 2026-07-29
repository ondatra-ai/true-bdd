package remote

// Finding 1 (full-request budget + out-of-band cannot-fit) and finding 3
// (newer-epoch-behind-pending-retry race) remote goldens.
//
// Finding 1 asserts the remote fits the SERIALIZED FULL InventoryRequest —
// snapshot PLUS the folder/session/epoch/token envelope — under the server's
// negotiated limit, including a long, JSON-escaped folder path whose
// escaping inflates the envelope; and that a server cap below the minimum
// viable request is a terminal cannot-fit state reported OUT OF BAND on the
// poll rather than a 413 wedge.
//
// Finding 3 asserts that when a newer refresh ticket arrives while an older
// pending upload is in flight, the successful older upload is followed by an
// immediate re-scan for the newer epoch, so a manual refresh is never
// consumed without its newer ticket ever being scanned.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ondatra-ai/true-bdd/src/internal/app/inventory"
)

// writeStoryFolder writes a canonical epic declaring n stories with ~kib KiB
// descriptions under root, so the full snapshot far exceeds a modest limit.
func writeStoryFolder(t *testing.T, root string, storyCount, kib int) {
	t.Helper()

	write := func(rel, content string) {
		path := filepath.Join(root, rel)

		mkdirErr := os.MkdirAll(filepath.Dir(path), 0o755)
		if mkdirErr != nil {
			t.Fatalf("mkdir %s: %v", rel, mkdirErr)
		}

		writeErr := os.WriteFile(path, []byte(content), 0o600)
		if writeErr != nil {
			t.Fatalf("write %s: %v", rel, writeErr)
		}
	}

	epic := &strings.Builder{}
	epic.WriteString("epic:\n  id: 70\n  name: Big Epic\nstories:\n")

	body := strings.Repeat("word ", kib*1024/5)

	for index := 1; index <= storyCount; index++ {
		id := fmt.Sprintf("70.%d", index)
		fmt.Fprintf(epic, "  - id: %q\n", id)
		write(fmt.Sprintf("docs/prd/stories/%s-big.yaml", id), fmt.Sprintf(
			"story:\n  id: %q\n  title: Big %d\n  as_a: user\n  i_want: x\n  so_that: y\n"+
				"  acceptance_criteria:\n    - id: AC-1\n      description: %q\n",
			id, index, body,
		))
	}

	write("docs/prd/epics/epic-70-big.yaml", epic.String())
}

// limitEnforcingServer records every inventory body length and rejects any
// body over `limit` with 413 (else 200), mirroring the real streamed cap.
func limitEnforcingServer(t *testing.T, limit int) (*httptest.Server, *[]int, *sync.Mutex) {
	t.Helper()

	var (
		guard sync.Mutex
		sizes []int
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/agent/inventory", func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)

		guard.Lock()

		sizes = append(sizes, len(body))
		guard.Unlock()

		if len(body) > limit {
			writer.WriteHeader(http.StatusRequestEntityTooLarge)

			return
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"generation":1}`))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return server, &sizes, &guard
}

func TestInventoryFullRequestFitsLimitWithEscapedFolder(t *testing.T) {
	t.Parallel()

	// A folder path with JSON-significant characters (quote, backslash, tab,
	// unicode) inflates the serialized envelope — the fit must account for it.
	root := t.TempDir()

	folder := filepath.Join(root, "weird \"q\" \\b\tc ünïcödé path")

	err := os.MkdirAll(folder, 0o755)
	if err != nil {
		t.Fatalf("mkdir escaped folder: %v", err)
	}

	writeStoryFolder(t, folder, 12, 20)

	const limit = 6000

	server, sizes, guard := limitEnforcingServer(t, limit)

	agent := &Agent{
		client:         NewServerClient(server.URL),
		folder:         folder,
		sessionID:      "sess-escaped",
		scanEpoch:      1,
		inventoryLimit: limit,
	}

	agent.uploadInventory(context.Background())

	guard.Lock()
	defer guard.Unlock()

	if len(*sizes) == 0 {
		t.Fatal("no inventory upload was attempted")
	}

	// A 413'd attempt is allowed mid-flight, but the loop must end with an
	// accepted body that fit the limit.
	accepted := false

	for _, size := range *sizes {
		if size <= limit {
			accepted = true
		}
	}

	last := (*sizes)[len(*sizes)-1]
	if last > limit {
		t.Fatalf("final inventory body = %d bytes, want <= limit %d", last, limit)
	}

	if !accepted {
		t.Fatalf("no inventory body fit the limit %d; sizes=%v", limit, *sizes)
	}

	if agent.getInventoryUnavailable() != "" {
		t.Fatalf("inventory reported unavailable(%q) despite a viable limit", agent.getInventoryUnavailable())
	}
}

func TestInventoryCapBelowMinimumIsTerminalAndReportedOutOfBand(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	writeStoryFolder(t, folder, 4, 4)

	// A cap far below the minimum viable request (floor snapshot + envelope).
	const tinyLimit = 32

	server, sizes, guard := limitEnforcingServer(t, tinyLimit)

	agent := &Agent{
		client:    NewServerClient(server.URL),
		folder:    folder,
		sessionID: "sess-tiny",
		scanEpoch: 1,
	}

	// Register-time validation of the server cap.
	agent.applyInventoryLimit(tinyLimit)

	if agent.getInventoryUnavailable() != inventory.UnavailableLimitTooSmall {
		t.Fatalf("cap below minimum: unavailable = %q, want %q",
			agent.getInventoryUnavailable(), inventory.UnavailableLimitTooSmall)
	}

	// uploadInventory must NOT hammer the route with impossible uploads.
	agent.uploadInventory(context.Background())

	guard.Lock()
	posts := len(*sizes)
	guard.Unlock()

	if posts != 0 {
		t.Fatalf("terminal cannot-fit still POSTed %d inventory upload(s), want 0", posts)
	}
}

func TestPollCarriesInventoryUnavailableOutOfBand(t *testing.T) {
	t.Parallel()

	var (
		guard    sync.Mutex
		reported string
		seen     bool
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/agent/poll", func(writer http.ResponseWriter, request *http.Request) {
		var req PollRequest

		_ = json.NewDecoder(request.Body).Decode(&req)

		guard.Lock()
		reported = req.InventoryUnavailable
		seen = true
		guard.Unlock()

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	agent := &Agent{
		client:               NewServerClient(server.URL),
		sessionID:            "sess-unavail",
		inventoryUnavailable: inventory.UnavailableLimitTooSmall,
	}

	runCh := make(chan RunSpec, 1)
	invCh := make(chan struct{}, 1)
	agent.pollOnce(context.Background(), runCh, invCh)

	guard.Lock()
	defer guard.Unlock()

	if !seen {
		t.Fatal("poll was not received by the server")
	}

	if reported != inventory.UnavailableLimitTooSmall {
		t.Fatalf("poll inventory_unavailable = %q, want %q", reported, inventory.UnavailableLimitTooSmall)
	}
}

func TestInventoryNewerEpochRescanAfterPendingRetry(t *testing.T) {
	t.Parallel()

	var (
		guard  sync.Mutex
		epochs []int
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/agent/inventory", func(writer http.ResponseWriter, request *http.Request) {
		var req InventoryRequest

		_ = json.NewDecoder(request.Body).Decode(&req)

		guard.Lock()

		epochs = append(epochs, req.ScanEpoch)
		guard.Unlock()

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"generation":1}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	folder := t.TempDir()
	writeStoryFolder(t, folder, 2, 1)

	agent := &Agent{
		client:    NewServerClient(server.URL),
		folder:    folder,
		sessionID: "sess-race",
		// A newer refresh ticket (epoch 2) has already been observed by the
		// poller while an older pending upload (epoch 1) awaits its retry.
		scanEpoch: 2,
		pendingInv: &InventoryRequest{
			SessionID:       "sess-race",
			CanonicalFolder: folder,
			ScanEpoch:       1,
			Snapshot:        inventory.Snapshot{},
			ClientToken:     "sess-race-inv-1",
		},
	}

	agent.uploadInventory(context.Background())

	guard.Lock()
	defer guard.Unlock()

	if len(epochs) < 2 {
		t.Fatalf("newer epoch was not re-scanned: server saw epochs %v, want [1 2]", epochs)
	}

	if epochs[0] != 1 {
		t.Fatalf("first upload epoch = %d, want the pending 1", epochs[0])
	}

	if epochs[len(epochs)-1] != 2 {
		t.Fatalf("last upload epoch = %d, want the newer 2 (re-scanned)", epochs[len(epochs)-1])
	}

	if agent.pendingInv != nil {
		t.Fatal("pending inventory not cleared after the newer-epoch upload was acked")
	}
}
