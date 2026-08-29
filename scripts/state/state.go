package state

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// The keys the repository's tooling shares. Named because they are a contract
// between packages that never call each other.
const (
	TaskKey    = "task"
	TicketKey  = "ticket"
	MandateKey = "mandate"
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

// Init removes the state file, which is how /task-start rolls a Task.
// Dropping the `log` key is what rolls the log, so no log file is removed —
// the previous Task's records are what TaskLog exists to keep readable.
func Init(repo string) error {
	return disk.Remove(File(repo))
}

// Get returns the last value written for key, or "" when nothing set it —
// which is also what a deleted key reads as.
func Get(repo, key string) string {
	return fold(repo)[key]
}

// Set appends one record. disk.Append is what keeps a concurrent Set from
// tearing a line: one write, newline included.
func Set(repo, key, value string) error {
	line, err := json.Marshal(record{K: key, V: value})
	if err != nil {
		return fmt.Errorf("encoding %s: %w", key, err)
	}

	return disk.Append(File(repo), line, disk.Private)
}

// fold replays the file. A line that does not parse is skipped rather than
// fatal: one bad record must not strand every key written after it.
func fold(repo string) map[string]string {
	values := map[string]string{}

	raw, err := disk.Read(File(repo))
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
