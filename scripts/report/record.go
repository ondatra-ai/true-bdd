package report

import (
	"encoding/json"
	"strings"
	"time"
)

// slog's own spellings of the two levels that decide a status.
const (
	levelError = "ERROR"
	levelWarn  = "WARN"
)

// Record is one line of the Task log, decoded to the fields the report reads.
// Every other attribute a writer stamps stays on disk, unread.
type Record struct {
	Time       time.Time `json:"time"`
	Level      string    `json:"level"`
	Msg        string    `json:"msg"`
	Tool       string    `json:"tool"`
	Run        string    `json:"run"`
	Tree       string    `json:"tree"`
	DurationMs *int64    `json:"duration_ms"`
	Status     Status    `json:"status"`
	Skipped    bool      `json:"skipped"`
}

// structured reports that the record says something about the run's shape:
// a marker, or a leaf carrying its own measurement.
func (r Record) structured() bool {
	return r.Tree != "" || r.DurationMs != nil
}

// decode reads the log and keeps one run's records — or every record, when
// run is empty. A line that does not parse is skipped rather than fatal,
// matching state's own fold: one torn record must not strand the rest.
func decode(raw []byte, run string) []Record {
	var records []Record

	for line := range strings.SplitSeq(string(raw), "\n") {
		var record Record

		if line == "" || json.Unmarshal([]byte(line), &record) != nil {
			continue
		}

		if run == "" || record.Run == run {
			records = append(records, record)
		}
	}

	return records
}
