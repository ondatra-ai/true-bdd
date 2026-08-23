package clickup

import (
	"encoding/json"
	"fmt"
	"os"
)

// Finding is one row of a queue file: a review finding on its way to becoming
// a ticket.
//
// One type for the whole pipeline, because one JSON shape travels it. The
// merge loop writes these to `tmp/merge/round-N/*.json`, hands the deferred
// ones to `file`, and the fix-queue skill reads what came back. Splitting it
// per stage would mean three structs that must agree about a file on disk.
//
// Absent and empty are not distinguished, which they were in Python: a queue
// row that carries `"file": ""` renders as `?` here where it rendered as
// empty before. No queue this repository has produced does that, and `?` is
// the better of the two answers.
type Finding struct {
	ID       string `json:"id"`
	Source   string `json:"source"`
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     string `json:"line"`
	Title    string `json:"title"`
	Body     string `json:"body"`

	// The GitHub handles of a thread finding. Null for a body-only one,
	// which is exactly what makes it unanswerable — see the merge loop.
	CommentID *int    `json:"comment_id"`
	ThreadID  *string `json:"thread_id"`
	Author    string  `json:"author,omitempty"`

	// Added by triage.
	Score  int    `json:"score,omitempty"`
	Reason string `json:"reason,omitempty"`

	// Added by a fix that ran.
	FixSummary   string   `json:"fix_summary,omitempty"`
	FilesChanged []string `json:"files_changed,omitempty"`
}

// orUnknown is the `finding.get(key, "?")` of the script this ports.
func orUnknown(value string) string {
	if value == "" {
		return "?"
	}

	return value
}

// LoadQueue reads a queue file.
func LoadQueue(path string) ([]Finding, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the path is an operator's argument.
	if err != nil {
		return nil, fmt.Errorf("reading the queue: %w", err)
	}

	var queue []Finding

	err = json.Unmarshal(raw, &queue)
	if err != nil {
		return nil, fmt.Errorf("parsing the queue %s: %w", path, err)
	}

	return queue, nil
}
