package clickup

import (
	"encoding/json"
	"fmt"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// Finding is one row of a queue file: a review finding on its way to
// becoming a ticket. One type for the whole pipeline (merge writes it, file
// hands it off, the rendered markdown shows it) — absent and empty both render `?`, unlike the Python it ports.
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
	raw, err := disk.Read(path)
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
