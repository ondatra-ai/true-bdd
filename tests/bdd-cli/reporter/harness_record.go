package reporter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/runner"
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

// applyHarnessOutcome fills in the four facts that exist only outside
// the engine's own process: the fixture's wall clock and verdict, the
// file diff, and what the judge spent.
//
// Per fixture, all-or-nothing: the harness record if it is there, else
// the legacy `go test` log if the caller opted into one. Never merged
// field by field — a wall clock from one source and a judge stamp from
// another manufactures a residual nobody can explain.
func applyHarnessOutcome(fixture *Fixture, dir string, gotest *GoTestLog) {
	if applyHarnessRecord(fixture, dir) {
		return
	}

	applyGoTestVerdict(fixture, gotest)
}

// applyHarnessRecord folds in the harness's own view of a fixture, and
// reports whether there was one.
//
// Absent or unreadable is not an error: a session recorded before the
// harness wrote this file still renders from the engine log alone, just
// without a wall clock, a verdict or a judge cost — the same degradation
// a missing `go test` log used to give.
func applyHarnessRecord(fixture *Fixture, dir string) bool {
	path := filepath.Join(dir, runner.SpawnLogDir, runner.HarnessRecordFile)

	blob, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	var record runner.HarnessRecord

	err = json.Unmarshal(blob, &record)
	if err != nil {
		return false
	}

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
