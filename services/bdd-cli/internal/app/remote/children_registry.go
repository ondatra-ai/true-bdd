package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// childEntry is one line in the remote's children pids file (JSONL): PGID is
// the signal target, start identity guards a recycled pid, and run id ties
// the entry to its run — these fields are also read by the legacy TS test harness's teardown (plan §3.2/§4.1).
type childEntry struct {
	PGID          int    `json:"pgid"`
	StartIdentity string `json:"start_identity,omitempty"`
	RunID         string `json:"run_id,omitempty"`
}

// ChildrenRegistry maintains <folder>/tmp/true-bdd-remote-children.pids as
// JSONL, one line per live command child, so scoped teardown can reap child
// process groups the remote owned even after a timeout (plan §4.1).
type ChildrenRegistry struct {
	path    string
	mu      sync.Mutex
	entries []childEntry
}

// NewChildrenRegistry builds a registry backed by path.
func NewChildrenRegistry(path string) *ChildrenRegistry {
	return &ChildrenRegistry{path: path}
}

// Add records a live child and rewrites the pids file.
func (r *ChildrenRegistry) Add(entry childEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = append(r.entries, entry)
	r.flush()
}

// Remove drops the child with the given pgid and rewrites the file.
func (r *ChildrenRegistry) Remove(pgid int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	kept := r.entries[:0]

	for _, entry := range r.entries {
		if entry.PGID != pgid {
			kept = append(kept, entry)
		}
	}

	r.entries = kept
	r.flush()
}

// flush rewrites the pids file from the in-memory entries and fsyncs it, so
// the recorded group identity is durable before the supervisor is released
// to run the mutating command (finding 4); errors are ignored — best-effort only.
func (r *ChildrenRegistry) flush() {
	var buffer bytes.Buffer

	for _, entry := range r.entries {
		line, err := json.Marshal(entry)
		if err != nil {
			continue
		}

		buffer.Write(line)
		buffer.WriteByte('\n')
	}

	_ = disk.Write(r.path, buffer.Bytes(), disk.Private)
}

// processStartIdentity returns `ps -o lstart=` for pid — a value stable
// for the process's life, mirroring the test-side identity check so a
// recycled pid is never signalled. Empty when ps fails.
func processStartIdentity(pid int) string {
	out, err := exec.CommandContext(context.Background(), "ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(out))
}
