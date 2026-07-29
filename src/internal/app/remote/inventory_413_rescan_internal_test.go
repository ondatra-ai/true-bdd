package remote

// Remote transport golden (plan §1a/§4 "Go"): a 413 from the inventory
// route must trigger a RE-SCAN against a strictly smaller budget (the
// negotiated server limit changed mid-flight, or the fit under-estimated),
// never a verbatim retry of the same oversized payload that would wedge
// forever. This drives ONE Agent.uploadInventory against a stub whose first
// inventory POST 413s and whose second succeeds, and asserts the remote
// sent a SECOND, STRICTLY SMALLER request.
//
// Compile-safe: uses only existing symbols (Agent, uploadInventory,
// inventory.Scan via the real folder). It is RED today because
// uploadInventory does not re-scan on 413 — it logs and returns after a
// single 413'd POST — and goes green once the fit-to-budget re-scan lands.

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// bigInventoryFolder writes a canonical epic declaring n stories, each with
// a ~20 KiB description, so the full snapshot far exceeds a tiny budget.
func bigInventoryFolder(t *testing.T, storyCount int) string {
	t.Helper()

	root := t.TempDir()
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

	body := strings.Repeat("word ", 4096)

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

	return root
}

func TestInventory413TriggersSmallerRescan(t *testing.T) {
	t.Parallel()

	var (
		guard sync.Mutex
		sizes []int
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/agent/inventory", func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)

		guard.Lock()
		attempt := len(sizes)
		sizes = append(sizes, len(body))
		guard.Unlock()

		// First upload is over the (tiny) budget → 413; the re-scanned,
		// smaller upload is accepted.
		if attempt == 0 {
			writer.WriteHeader(http.StatusRequestEntityTooLarge)

			return
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"generation":1}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	agent := &Agent{
		client:    NewServerClient(server.URL),
		folder:    bigInventoryFolder(t, 8),
		sessionID: "sess-413",
		scanEpoch: 1,
	}

	agent.uploadInventory(context.Background())

	guard.Lock()
	defer guard.Unlock()

	if len(sizes) < 2 {
		t.Fatalf("413 did not trigger a re-scan: server saw %d inventory POST(s), want >= 2", len(sizes))
	}

	if sizes[1] >= sizes[0] {
		t.Fatalf("re-scan was not strictly smaller: first=%d bytes, second=%d bytes", sizes[0], sizes[1])
	}
}
