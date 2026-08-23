package remote

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestChildrenRegistryPidsFileFormat proves the pids file the scoped
// teardown reads (plan §3.2/§4.1) carries PGID + start identity + run id per
// entry, one JSONL object per live child, and that Remove drops an entry.
func TestChildrenRegistryPidsFileFormat(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "true-bdd-remote-children.pids")
	registry := NewChildrenRegistry(path)

	registry.Add(childEntry{PGID: 4242, StartIdentity: "Wed Jul 29 14:00:00 2026", RunID: testRunID})
	registry.Add(childEntry{PGID: 4343, StartIdentity: "Wed Jul 29 14:01:00 2026", RunID: "run-2"})

	entries := readPidLines(t, path)
	if len(entries) != 2 {
		t.Fatalf("got %d pids entries, want 2", len(entries))
	}

	if entries[0].PGID != 4242 || entries[0].StartIdentity == "" || entries[0].RunID != testRunID {
		t.Fatalf("first entry = %+v, want pgid+start_identity+run_id", entries[0])
	}

	registry.Remove(4242)

	remaining := readPidLines(t, path)
	if len(remaining) != 1 || remaining[0].PGID != 4343 {
		t.Fatalf("after Remove, entries = %+v, want only pgid 4343", remaining)
	}
}

// readPidLines parses the JSONL pids file into childEntry values, using the
// same snake_case wire keys remote-process.ts consumes.
func readPidLines(t *testing.T, path string) []childEntry {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pids file: %v", err)
	}

	var entries []childEntry

	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var entry childEntry

		err = json.Unmarshal([]byte(line), &entry)
		if err != nil {
			t.Fatalf("parse pids line %q: %v", line, err)
		}

		entries = append(entries, entry)
	}

	return entries
}
