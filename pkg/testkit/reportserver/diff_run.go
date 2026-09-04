package reportserver

import (
	"slices"
	"znkr.io/diff"
)

// Edit operation names on the wire.
const (
	opMatch  = "match"
	opDelete = "delete"
	opInsert = "insert"
)

// RunComparison is two runs aligned test by test.
type RunComparison struct {
	Version int64               `json:"version"`
	Left    RunSummary          `json:"left"`
	Right   RunSummary          `json:"right"`
	Rows    []TestComparisonRow `json:"rows"`
	Summary CompareTotals       `json:"summary"`
}

// CompareTotals counts what the alignment found, so the UI can lead with
// "3 changed, 1 added" instead of making the reader scan for it.
type CompareTotals struct {
	Matched   int `json:"matched"`
	Changed   int `json:"changed"`
	OnlyLeft  int `json:"only_left"`
	OnlyRight int `json:"only_right"`
}

// TestComparisonRow is one test across both runs. Op says whether it was
// present in both; Changed says whether anything about it moved.
type TestComparisonRow struct {
	Op      string       `json:"op"`
	Name    string       `json:"name"`
	Left    *TestSummary `json:"left"`
	Right   *TestSummary `json:"right"`
	Changed bool         `json:"changed"`
	// Fields names what differs, so the UI highlights the cells that
	// moved rather than the whole row.
	Fields []string `json:"fields"`
	Deltas Deltas   `json:"deltas"`
}

// Deltas is right-minus-left for the numbers worth watching.
type Deltas struct {
	WallSeconds float64 `json:"wall_seconds"`
	CostUSD     float64 `json:"cost_usd"`
	Tokens      int     `json:"tokens"`
	Turns       int     `json:"turns"`
	DiffCount   int     `json:"diff_count"`
}

// CompareRuns aligns two runs' fixtures with a real sequence diff — a merge
// join would agree on today's sorted data, but silently mismatch rows on a
// rename or reordering, and the real algorithm costs three lines.
func CompareRuns(version int64, left, right *Run) RunComparison {
	comparison := RunComparison{
		Version: version,
		Left:    mapRunSummary(left),
		Right:   mapRunSummary(right),
		Rows:    []TestComparisonRow{},
	}

	for _, edit := range diff.Edits(testKeys(left.Tests), testKeys(right.Tests)) {
		row := TestComparisonRow{Fields: []string{}}

		switch edit.Op {
		case diff.Match:
			row.Op = opMatch
			row.Name = edit.X

			leftSummary := mapTestSummary(left.Tests[edit.PosX])
			rightSummary := mapTestSummary(right.Tests[edit.PosY])
			row.Left, row.Right = &leftSummary, &rightSummary
			row.Fields = changedFields(leftSummary, rightSummary)
			row.Changed = len(row.Fields) > 0
			row.Deltas = deltasOf(leftSummary, rightSummary)

			comparison.Summary.Matched++

			if row.Changed {
				comparison.Summary.Changed++
			}
		case diff.Delete:
			row.Op = opDelete
			row.Name = edit.X
			leftSummary := mapTestSummary(left.Tests[edit.PosX])
			row.Left = &leftSummary
			row.Changed = true

			comparison.Summary.OnlyLeft++
		case diff.Insert:
			row.Op = opInsert
			row.Name = edit.Y
			rightSummary := mapTestSummary(right.Tests[edit.PosY])
			row.Right = &rightSummary
			row.Changed = true

			comparison.Summary.OnlyRight++
		}

		comparison.Rows = append(comparison.Rows, row)
	}

	return comparison
}

// changedFields names what moved between two runs of the same test.
// Durations and costs are excluded on purpose: they differ on every
// model-driven run, which would mark every row changed. They travel as deltas instead.
func changedFields(left, right TestSummary) []string {
	fields := []string{}

	if left.Verdict != right.Verdict {
		fields = append(fields, "verdict")
	}

	if !samePointer(left.ExitCode, right.ExitCode) {
		fields = append(fields, "exit_code")
	}

	if left.Turns != right.Turns {
		fields = append(fields, "turns")
	}

	if left.DiffCount != right.DiffCount {
		fields = append(fields, "diff_count")
	}

	// Content, not just count: two runs can each fail one check for
	// different reasons, and comparing lengths alone would call that unchanged.
	if !slices.Equal(left.Failures, right.Failures) {
		fields = append(fields, "failures")
	}

	if left.Command != right.Command {
		fields = append(fields, "command")
	}

	return fields
}

// samePointer compares two optional exit codes, treating both-absent as
// equal and one-absent as different.
func samePointer(left, right *int) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}

	return *left == *right
}

// deltasOf is right minus left.
func deltasOf(left, right TestSummary) Deltas {
	return Deltas{
		WallSeconds: right.WallSeconds - left.WallSeconds,
		CostUSD:     right.CostUSD - left.CostUSD,
		Tokens:      right.Tokens - left.Tokens,
		Turns:       right.Turns - left.Turns,
		DiffCount:   right.DiffCount - left.DiffCount,
	}
}
