package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The keys the repository's tooling shares. Named because they are a contract
// between packages that never call each other.
const (
	TaskKey    = "task"
	TicketKey  = "ticket"
	MandateKey = "mandate"
)

// Permissions for the history tree: a directory a person browses, files only
// this tooling writes.
const (
	dirMode  = 0o755
	fileMode = 0o600
)

// record is one line of the file.
type record struct {
	K string `json:"k"`
	V string `json:"v"`
}

// HistoryDir holds the state file, the Task's transcript and its log. It is
// gitignored, so none of the three ever reaches a commit.
func HistoryDir(repo string) string {
	return filepath.Join(repo, "docs", "history")
}

// File is the one state file.
func File(repo string) string {
	return filepath.Join(HistoryDir(repo), "state.jsonl")
}

// Init removes the file. Whatever the last Task left goes with it.
func Init(repo string) error {
	err := os.Remove(File(repo))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", File(repo), err)
	}

	return nil
}

// Get returns the last value written for key, or "" when nothing set it —
// which is also what a deleted key reads as.
func Get(repo, key string) string {
	return fold(repo)[key]
}

// Set appends one record in ONE write, newline included: a second write for
// the newline could interleave with a concurrent Set and tear both lines.
func Set(repo, key, value string) error {
	line, err := json.Marshal(record{K: key, V: value})
	if err != nil {
		return fmt.Errorf("encoding %s: %w", key, err)
	}

	err = os.MkdirAll(HistoryDir(repo), dirMode)
	if err != nil {
		return fmt.Errorf("creating %s: %w", HistoryDir(repo), err)
	}

	handle, err := os.OpenFile(File(repo), os.O_APPEND|os.O_CREATE|os.O_WRONLY, fileMode)
	if err != nil {
		return fmt.Errorf("opening %s: %w", File(repo), err)
	}

	defer handle.Close() //nolint:errcheck // the write below reports the failure that matters.

	_, err = handle.Write(append(line, '\n'))
	if err != nil {
		return fmt.Errorf("appending to %s: %w", File(repo), err)
	}

	return nil
}

// fold replays the file. A line that does not parse is skipped rather than
// fatal: one bad record must not strand every key written after it.
func fold(repo string) map[string]string {
	values := map[string]string{}

	raw, err := os.ReadFile(File(repo))
	if err != nil {
		return values
	}

	for _, line := range strings.Split(string(raw), "\n") {
		var entry record

		if line == "" || json.Unmarshal([]byte(line), &entry) != nil {
			continue
		}

		values[entry.K] = entry.V
	}

	return values
}
