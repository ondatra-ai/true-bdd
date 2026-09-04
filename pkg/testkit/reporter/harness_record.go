package reporter

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

// JudgeCall is one harness judge invocation. At is when the call FINISHED —
// phase.go measures the trailing harness block as the span from the
// engine's last log record to this instant, the far edge of the judge.
type JudgeCall struct {
	At      time.Time
	CostUSD float64
	Tokens  int
}

// HarnessRecordPath is where a fixture's harness record lives — the last
// byte ever written into a fixture directory (HarnessRecorder.Finish runs
// last via t.Cleanup's LIFO), so its presence means the fixture is final.
func HarnessRecordPath(dir string) string {
	return filepath.Join(dir, runner.SpawnLogDir, runner.HarnessRecordFile)
}

// applyHarnessRecord folds in the harness's own view of a fixture, and
// reports whether there was one. Absent or unreadable is not an error: it
// still renders from the engine log alone, minus wall clock, verdict and judge cost.
func applyHarnessRecord(fixture *Fixture, dir string) bool {
	blob, err := disk.Read(HarnessRecordPath(dir))
	if err != nil {
		return false
	}

	var record runner.HarnessRecord

	err = json.Unmarshal(blob, &record)
	if err != nil {
		return false
	}

	// Carried whole, not field by field: copying exit code, timestamps,
	// diff and judge model out one at a time is how those fields got lost
	// before, since none of them showed in the HTML report to catch it.
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
