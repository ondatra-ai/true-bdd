package reporter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ondatra-ai/true-bdd/tests/libraries/runner"
)

// JudgeCall is one harness judge invocation.
//
// At is when the call FINISHED. phase.go measures the trailing harness
// block as the span from the engine's last log record to this instant,
// so it has to be the far edge of the judge, not the near one.
type JudgeCall struct {
	At      time.Time
	CostUSD float64
	Tokens  int
}

// HarnessRecordPath is where a fixture's harness record lives.
//
// It is the last byte ever written into a fixture directory —
// HarnessRecorder.Finish is registered first in the subtest, so
// t.Cleanup's LIFO runs it last. Its presence therefore means the
// fixture is final, which is exactly the cache key the report server
// needs.
func HarnessRecordPath(dir string) string {
	return filepath.Join(dir, runner.SpawnLogDir, runner.HarnessRecordFile)
}

// applyHarnessRecord folds in the harness's own view of a fixture, and
// reports whether there was one.
//
// Absent or unreadable is not an error: a fixture whose record has not
// landed yet — or is being written as we read — still renders from the
// engine log alone, just without a wall clock, a verdict or a judge
// cost, and the next scan picks up the rest.
func applyHarnessRecord(fixture *Fixture, dir string) bool {
	blob, err := os.ReadFile(HarnessRecordPath(dir))
	if err != nil {
		return false
	}

	var record runner.HarnessRecord

	err = json.Unmarshal(blob, &record)
	if err != nil {
		return false
	}

	// Carried whole rather than field by field: the server serves exit
	// code, timestamps, the structural diff and the judge's model, none
	// of which the HTML report ever showed, and copying them out one at
	// a time is how those fields got lost in the first place.
	fixture.Record = &record

	fixture.Wall = time.Duration(record.WallMs) * time.Millisecond
	fixture.HasWall = record.WallMs > 0
	fixture.HasRecord = true
	fixture.Verdict = record.Verdict
	fixture.Failures = record.Failures
	fixture.Diff = diffLines(record.Diff)

	if !record.Judge.EndedAt.IsZero() {
		fixture.Judge = &JudgeCall{
			At:      record.Judge.EndedAt,
			CostUSD: record.Judge.CostUSD,
			Tokens:  record.Judge.Tokens,
		}
	}

	return true
}

// diffLines renders the structural diff the way the report has always
// shown it, so the page text does not change with the source.
func diffLines(changes []runner.FileChangeRecord) []string {
	if len(changes) == 0 {
		return nil
	}

	lines := make([]string, 0, len(changes))

	for _, change := range changes {
		lines = append(lines, fmt.Sprintf("%s %s (%d bytes)",
			change.Kind, change.Path, change.BytesAfter))
	}

	return lines
}
