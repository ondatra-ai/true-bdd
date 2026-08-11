package reportserver

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/reporter"
)

// turn builds a turn in a checklist cell. The iteration index is set but
// deliberately never reaches the alignment key.
func turn(section, subject, role string) *reporter.Turn {
	return &reporter.Turn{
		Role: role,
		Cell: reporter.ChecklistCell{Index: "99", Subject: subject, Section: section},
	}
}

// happyPath is the real shape of us-create-happy-path: nine turns over
// four cells, several of them retries.
func happyPath() []*reporter.Turn {
	return []*reporter.Turn{
		turn("us-create-format", "99.1", "prompt"),
		turn("us-create-format", "99.1", "prompt"),
		turn("us-create-who", "99.1", "prompt"),
		turn("us-create-what", "99.1", "prompt"),
		turn("us-create-what", "99.1", "prompt"),
		turn("us-create-why", "99.1", "prompt"),
		turn("us-create-why", "99.1", "prompt"),
		turn("us-create-why", "99.1", "prompt"),
		turn("us-create-why", "99.1", "prompt"),
	}
}

// ops projects a comparison onto its operation sequence.
func ops(rows []TurnComparison) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Op)
	}

	return out
}

// TestIdenticalTurnsAllMatch is the baseline: the same sequence twice
// must produce nothing but matches.
func TestIdenticalTurnsAllMatch(t *testing.T) {
	rows := compareTurns(happyPath(), happyPath())

	if len(rows) != 9 {
		t.Fatalf("rows = %d, want 9", len(rows))
	}

	for index, row := range rows {
		if row.Op != opMatch {
			t.Errorf("row %d = %q, want match", index, row.Op)
		}
	}
}

// TestExtraRetryIsOneInsert is the case ordinal pairing gets wrong, and
// the reason this uses a real sequence diff.
//
// One extra attempt at `format` early in the run must show up as a
// single insertion inside that cell. Pairing turn-1-to-turn-1 would
// instead shift every later turn by one and report all eight remaining
// rows as changed, hiding the one real difference in the noise.
func TestExtraRetryIsOneInsert(t *testing.T) {
	left := happyPath()

	right := happyPath()
	// Insert a third `format` attempt at position 2.
	right = append(right[:2],
		append([]*reporter.Turn{turn("us-create-format", "99.1", "prompt")}, right[2:]...)...)

	rows := compareTurns(left, right)

	inserts := 0

	for _, row := range rows {
		if row.Op == opInsert {
			inserts++
		}

		if row.Op == opDelete {
			t.Errorf("unexpected delete at %q — nothing was removed", row.Label)
		}
	}

	if inserts != 1 {
		t.Errorf("inserts = %d, want exactly 1", inserts)
	}

	if len(rows) != 10 {
		t.Errorf("rows = %d, want 10 (9 matched + 1 inserted)", len(rows))
	}

	// Every one of the original nine still matched: the later cells did
	// not shift.
	matched := 0

	for _, row := range rows {
		if row.Op == opMatch {
			matched++
		}
	}

	if matched != 9 {
		t.Errorf("matched = %d, want 9 — the insertion shifted later turns", matched)
	}
}

// TestRolesDoNotCollapse pins that the three turns of one cell stay
// distinct: a fix turn must never align against the prompt turn that
// preceded it.
func TestRolesDoNotCollapse(t *testing.T) {
	left := []*reporter.Turn{
		turn("merge", "99.3", "prompt"),
		turn("merge", "99.3", "fix"),
		turn("merge", "99.3", "apply"),
	}
	right := []*reporter.Turn{
		turn("merge", "99.3", "prompt"),
		turn("merge", "99.3", "apply"),
	}

	rows := compareTurns(left, right)

	if got := ops(rows); strings.Join(got, ",") != "match,delete,match" {
		t.Errorf("ops = %v, want [match delete match] — the fix turn must be the one dropped", got)
	}
}

// TestEveryElementAppearsExactlyOnce is the structural invariant of any
// sequence diff, asserted instead of a golden: the library documents its
// output as unstable across minor versions, so pinning exact edit
// streams would break on an upgrade that is still correct.
func TestEveryElementAppearsExactlyOnce(t *testing.T) {
	left := happyPath()
	right := []*reporter.Turn{
		turn("us-create-format", "99.1", "prompt"),
		turn("us-create-why", "99.1", "prompt"),
		turn("us-refine-shape", "99.2", "fix"),
	}

	rows := compareTurns(left, right)

	fromLeft, fromRight := accountFor(t, rows)

	if fromLeft != len(left) {
		t.Errorf("left elements accounted for = %d, want %d", fromLeft, len(left))
	}

	if fromRight != len(right) {
		t.Errorf("right elements accounted for = %d, want %d", fromRight, len(right))
	}
}

// accountFor counts how many elements of each side the edit stream
// claims, checking that every row carries exactly the sides its op
// implies.
func accountFor(t *testing.T, rows []TurnComparison) (int, int) {
	t.Helper()

	fromLeft, fromRight := 0, 0

	for _, row := range rows {
		switch row.Op {
		case opMatch:
			fromLeft++
			fromRight++

			if row.Left == nil || row.Right == nil {
				t.Error("a match must carry both sides")
			}
		case opDelete:
			fromLeft++

			if row.Right != nil {
				t.Error("a delete must not carry a right side")
			}
		case opInsert:
			fromRight++

			if row.Left != nil {
				t.Error("an insert must not carry a left side")
			}
		}
	}

	return fromLeft, fromRight
}

// TestFilePathsAlignOnIdentityNotRenderedLine pins that a file which
// merely changed size is one matched row, not a delete plus an insert.
func TestFilePathsAlignOnIdentityNotRenderedLine(t *testing.T) {
	left := []FileChangeDTO{{Kind: "created", Path: "a.yaml", BytesAfter: 100}}
	right := []FileChangeDTO{{Kind: "created", Path: "a.yaml", BytesAfter: 250}}

	rows := compareFiles(left, right)

	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1 — a resized file is not a delete plus an insert", len(rows))
	}

	if rows[0].Op != opMatch {
		t.Errorf("op = %q, want match", rows[0].Op)
	}

	if !rows[0].Changed {
		t.Error("a file whose size moved should be flagged changed")
	}
}

// TestTextDiffReportsIdenticalAndTruncation pins the two claims a reader
// acts on: "these are the same", and "there is more than this".
func TestTextDiffReportsIdenticalAndTruncation(t *testing.T) {
	same := DiffText("alpha\nbeta\n", "alpha\nbeta\n", 3)
	if !same.Identical || len(same.Hunks) != 0 {
		t.Errorf("identical inputs produced %d hunk(s), identical=%v", len(same.Hunks), same.Identical)
	}

	// Two entirely unrelated texts are the pathological case: hunks cover
	// both sides completely.
	var left, right strings.Builder

	for index := range maxDiffEdits {
		left.WriteString("left line ")
		left.WriteString(itoa(index))
		left.WriteString("\n")
		right.WriteString("right line ")
		right.WriteString(itoa(index))
		right.WriteString("\n")
	}

	big := DiffText(left.String(), right.String(), 3)
	if !big.Truncated {
		t.Error("a diff past the edit cap must say so rather than look complete")
	}
}

// TestChangedFieldsIgnoreTimings pins that duration and cost are not
// "changes". They differ on every model-driven run, so flagging them
// would mark all 19 rows changed and make the signal worthless.
func TestChangedFieldsIgnoreTimings(t *testing.T) {
	left := TestSummary{Name: "fx", Verdict: "PASS", WallSeconds: 100, CostUSD: 1.0, Tokens: 10}
	right := TestSummary{Name: "fx", Verdict: "PASS", WallSeconds: 250, CostUSD: 3.0, Tokens: 90}

	if fields := changedFields(left, right); len(fields) != 0 {
		t.Errorf("fields = %v, want none — timings are deltas, not changes", fields)
	}

	right.Verdict = "FAIL"
	if fields := changedFields(left, right); len(fields) != 1 || fields[0] != "verdict" {
		t.Errorf("fields = %v, want [verdict]", changedFields(left, right))
	}
}
