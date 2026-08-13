package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

const childrenFilePerm = 0o600

// childEntry is one line in the remote's children pids file. The test
// harness's teardown reads exactly these fields (plan §3.2 / §4.1): the
// PGID is the signal target, the start identity guards against a
// recycled pid, and the run id ties the entry to its run.
type childEntry struct {
	PGID          int    `json:"pgid"`
	StartIdentity string `json:"start_identity,omitempty"`
	RunID         string `json:"run_id,omitempty"`
}

// ChildrenRegistry maintains <folder>/tmp/true-bdd-remote-children.pids
// as JSONL — one line per live command child. The file lets scoped
// teardown reap child process groups the remote owned even after a
// timeout (plan §4.1).
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

// flush rewrites the pids file from the in-memory entries and fsyncs it, so the
// recorded group identity is durable before the supervisor is released to run
// the mutating command (finding 4). Best-effort: a write failure only degrades
// emergency teardown hygiene.
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

	file, err := os.OpenFile(r.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, childrenFilePerm)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	_, err = file.Write(buffer.Bytes())
	if err != nil {
		return
	}

	_ = file.Sync()
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
