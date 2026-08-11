package reportserver

import (
	"znkr.io/diff"

	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/reporter"
)

// TestComparison is one test across two runs: its expectations, its
// outcome, its turns and the files it touched, each aligned.
type TestComparison struct {
	Version  int64            `json:"version"`
	Name     string           `json:"name"`
	LeftRun  string           `json:"left_run"`
	RightRun string           `json:"right_run"`
	Left     TestSummary      `json:"left"`
	Right    TestSummary      `json:"right"`
	Expected ExpectedDiff     `json:"expected"`
	Actual   ActualDiff       `json:"actual"`
	Judge    JudgeDiff        `json:"judge"`
	Turns    []TurnComparison `json:"turns"`
	Files    []FileComparison `json:"files"`
}

// ScalarDiff is one labelled field side by side.
type ScalarDiff struct {
	Field   string `json:"field"`
	Left    string `json:"left"`
	Right   string `json:"right"`
	Changed bool   `json:"changed"`
}

// ListEdit is one element of an aligned string list.
type ListEdit struct {
	Op    string `json:"op"`
	Value string `json:"value"`
}

// ExpectedDiff compares what the two runs were asked to satisfy.
//
// This is only meaningful because the harness snapshots each run's
// manifest: read from the source tree, both sides would always be
// today's fixture.yaml and every comparison would show no change.
// SourceLeft/SourceRight say which side is a real snapshot.
type ExpectedDiff struct {
	SourceLeft   string       `json:"source_left"`
	SourceRight  string       `json:"source_right"`
	Comparable   bool         `json:"comparable"`
	Scalars      []ScalarDiff `json:"scalars"`
	StdoutChecks []ListEdit   `json:"stdout_checks"`
	Prep         []ListEdit   `json:"prep"`
	Teardown     []ListEdit   `json:"teardown"`
	JudgeSpec    []ListEdit   `json:"judge_spec"`
}

// ActualDiff compares what the two runs produced.
type ActualDiff struct {
	Scalars  []ScalarDiff `json:"scalars"`
	Failures []ListEdit   `json:"failures"`
}

// JudgeDiff says whether the two judge transcripts can be diffed, and
// carries the refs to do it with.
type JudgeDiff struct {
	LeftHasTranscript  bool         `json:"left_has_transcript"`
	RightHasTranscript bool         `json:"right_has_transcript"`
	Comparable         bool         `json:"comparable"`
	Scalars            []ScalarDiff `json:"scalars"`
}

// TurnComparison is one aligned turn pair.
type TurnComparison struct {
	Op      string     `json:"op"`
	CellKey string     `json:"cell_key"`
	Label   string     `json:"label"`
	Left    *TurnDTO   `json:"left"`
	Right   *TurnDTO   `json:"right"`
	Changed bool       `json:"changed"`
	Deltas  TurnDeltas `json:"deltas"`
}

// TurnDeltas is right minus left for one turn.
type TurnDeltas struct {
	DurationSeconds float64 `json:"duration_seconds"`
	CostUSD         float64 `json:"cost_usd"`
	Tokens          int     `json:"tokens"`
}

// FileComparison is one aligned file-change pair.
type FileComparison struct {
	Op      string         `json:"op"`
	Path    string         `json:"path"`
	Left    *FileChangeDTO `json:"left"`
	Right   *FileChangeDTO `json:"right"`
	Changed bool           `json:"changed"`
}

// CompareTests aligns one fixture across two runs.
func CompareTests(version int64, leftRun, rightRun string, left, right *reporter.Fixture) TestComparison {
	comparison := TestComparison{
		Version:  version,
		Name:     left.Name,
		LeftRun:  leftRun,
		RightRun: rightRun,
		Left:     mapTestSummary(left),
		Right:    mapTestSummary(right),
		Expected: compareExpected(left.Manifest, right.Manifest),
		Actual:   compareActual(left, right),
		Judge:    compareJudge(left, right),
		Turns:    compareTurns(left, right, left.Turns, right.Turns),
		Files:    compareFiles(mapFiles(left), mapFiles(right)),
	}

	return comparison
}

// compareExpected diffs the two manifests.
func compareExpected(left, right *reporter.Manifest) ExpectedDiff {
	expected := ExpectedDiff{
		SourceLeft:  sourceOf(left),
		SourceRight: sourceOf(right),
	}

	// Only a real per-run snapshot on both sides makes this a comparison
	// of what the runs were actually held to. Two repo reads are the same
	// bytes twice, and saying "no change" about them would be a lie of
	// omission.
	expected.Comparable = expected.SourceLeft == string(reporter.ManifestSnapshot) &&
		expected.SourceRight == string(reporter.ManifestSnapshot)

	// Sources are captured above, before either side is replaced: an
	// absent manifest must still report "absent" rather than read as an
	// empty snapshot. Everything below dereferences, so substitute now.
	if left == nil {
		left = &reporter.Manifest{}
	}

	if right == nil {
		right = &reporter.Manifest{}
	}

	expected.Scalars = []ScalarDiff{
		scalar("command", left.Command, right.Command),
		scalar("exit_code", itoa(left.ExpectedExit), itoa(right.ExpectedExit)),
		scalar("answers", left.Answers, right.Answers),
		scalar("input_path", left.InputPath, right.InputPath),
	}

	expected.StdoutChecks = diffStrings(left.StdoutChecks, right.StdoutChecks)
	expected.Prep = diffStrings(left.PrepCmds, right.PrepCmds)
	expected.Teardown = diffStrings(left.TeardownCmds, right.TeardownCmds)
	expected.JudgeSpec = diffStrings(splitLines(left.JudgeSpec), splitLines(right.JudgeSpec))

	return expected
}

// sourceOf names where a manifest came from.
func sourceOf(manifest *reporter.Manifest) string {
	return string(manifestSource(manifest))
}

// compareActual diffs the two outcomes.
func compareActual(left, right *reporter.Fixture) ActualDiff {
	return ActualDiff{
		Scalars: []ScalarDiff{
			scalar("verdict", left.Verdict, right.Verdict),
			scalar("exit_code", exitCodeOf(left), exitCodeOf(right)),
			scalar("turns", itoa(len(left.Turns)), itoa(len(right.Turns))),
			scalar("files_changed", itoa(len(mapFiles(left))), itoa(len(mapFiles(right)))),
		},
		Failures: diffStrings(left.Failures, right.Failures),
	}
}

// exitCodeOf renders a fixture's exit code, or "—" when it never exited.
func exitCodeOf(fixture *reporter.Fixture) string {
	if fixture.Record == nil || fixture.Record.ExitCode == nil {
		return "—"
	}

	return itoa(*fixture.Record.ExitCode)
}

// compareJudge reports whether the two judge calls can be diffed.
func compareJudge(left, right *reporter.Fixture) JudgeDiff {
	leftJudge, rightJudge := mapJudge(left), mapJudge(right)

	return JudgeDiff{
		LeftHasTranscript:  leftJudge.HasTranscript,
		RightHasTranscript: rightJudge.HasTranscript,
		Comparable:         leftJudge.HasTranscript && rightJudge.HasTranscript,
		Scalars: []ScalarDiff{
			scalar("model", leftJudge.Model, rightJudge.Model),
			scalar("tokens", itoa(leftJudge.Tokens), itoa(rightJudge.Tokens)),
		},
	}
}

// compareTurns aligns two turn sequences on cell identity.
func compareTurns(leftFixture, rightFixture *reporter.Fixture, left, right []*reporter.Turn) []TurnComparison {
	edits := diff.Edits(turnKeys(left), turnKeys(right))
	rows := make([]TurnComparison, 0, len(edits))

	for _, edit := range edits {
		row := TurnComparison{}

		switch edit.Op {
		case diff.Match:
			row.Op = opMatch
			row.CellKey = edit.X

			leftTurn := mapTurn(leftFixture, edit.PosX, left[edit.PosX])
			rightTurn := mapTurn(rightFixture, edit.PosY, right[edit.PosY])
			row.Left, row.Right = &leftTurn, &rightTurn
			row.Label = leftTurn.Operation.CellLabel
			row.Deltas = TurnDeltas{
				DurationSeconds: rightTurn.DurationSeconds - leftTurn.DurationSeconds,
				CostUSD:         rightTurn.CostUSD - leftTurn.CostUSD,
				Tokens:          rightTurn.Tokens.Total - leftTurn.Tokens.Total,
			}
			row.Changed = leftTurn.Status != rightTurn.Status || leftTurn.Model != rightTurn.Model
		case diff.Delete:
			row.Op = opDelete
			row.CellKey = edit.X
			leftTurn := mapTurn(leftFixture, edit.PosX, left[edit.PosX])
			row.Left = &leftTurn
			row.Label = leftTurn.Operation.CellLabel
			row.Changed = true
		case diff.Insert:
			row.Op = opInsert
			row.CellKey = edit.Y
			rightTurn := mapTurn(rightFixture, edit.PosY, right[edit.PosY])
			row.Right = &rightTurn
			row.Label = rightTurn.Operation.CellLabel
			row.Changed = true
		}

		rows = append(rows, row)
	}

	return rows
}

// compareFiles aligns two file-change lists on path, then compares the
// payload of the pairs that matched.
func compareFiles(left, right []FileChangeDTO) []FileComparison {
	edits := diff.Edits(filePaths(left), filePaths(right))
	rows := make([]FileComparison, 0, len(edits))

	for _, edit := range edits {
		row := FileComparison{}

		switch edit.Op {
		case diff.Match:
			row.Op = opMatch
			row.Path = edit.X
			leftFile, rightFile := left[edit.PosX], right[edit.PosY]
			row.Left, row.Right = &leftFile, &rightFile
			row.Changed = leftFile.Kind != rightFile.Kind ||
				leftFile.BytesAfter != rightFile.BytesAfter
		case diff.Delete:
			row.Op = opDelete
			row.Path = edit.X
			leftFile := left[edit.PosX]
			row.Left = &leftFile
			row.Changed = true
		case diff.Insert:
			row.Op = opInsert
			row.Path = edit.Y
			rightFile := right[edit.PosY]
			row.Right = &rightFile
			row.Changed = true
		}

		rows = append(rows, row)
	}

	return rows
}

// diffStrings aligns two string lists into a flat edit stream.
func diffStrings(left, right []string) []ListEdit {
	aligned := diff.Edits(left, right)
	edits := make([]ListEdit, 0, len(aligned))

	for _, edit := range aligned {
		switch edit.Op {
		case diff.Match:
			edits = append(edits, ListEdit{Op: opMatch, Value: edit.X})
		case diff.Delete:
			edits = append(edits, ListEdit{Op: opDelete, Value: edit.X})
		case diff.Insert:
			edits = append(edits, ListEdit{Op: opInsert, Value: edit.Y})
		}
	}

	return edits
}

// scalar builds one side-by-side field.
func scalar(field, left, right string) ScalarDiff {
	return ScalarDiff{Field: field, Left: left, Right: right, Changed: left != right}
}
