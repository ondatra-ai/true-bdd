package clickup_test

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
)

// The two globs the table repeats: one nested tree, one repository-wide.
const (
	mergeGlob = "scripts/merge/**"
	rootGlob  = "./*"
)

// The stamp the tests pass in place of the clock and HEAD.
const (
	stampMillis = 1787999823623
	stampCommit = "eeb67aa1c0de5f0e2a1b3c4d5e6f708192a3b4c5"
)

// TestPlanFieldsDerivesTheGlobAndTheIndex pins the two derivations the filing
// turn transcribes: an exact file becomes its directory's glob, and a score
// becomes the dropdown POSITION one below it.
func TestPlanFieldsDerivesTheGlobAndTheIndex(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		file  string
		score int
		glob  string
		index string
	}{
		{"nested file", "scripts/merge/parse.go", 7, mergeGlob, "6"},
		{"skill markdown", ".claude/skills/pr-merge/SKILL.md", 10, ".claude/skills/pr-merge/**", "9"},
		{"lowest score", "scripts/clickup/list.go", 1, "scripts/clickup/**", "0"},
		{"repository root", "CLAUDE.md", 5, rootGlob, "4"},
		// No file bounded nothing, so the row omits the field and task-handle
		// halts until a person fills it. `?` is what a queue row carries when
		// the reviewer named none.
		{"no file", "?", 4, "", "3"},
		{"empty file", "", 4, "", "3"},
		// An unscored finding leaves the dropdown alone rather than claiming
		// the bottom of the band, so the row omits the key entirely.
		{"unscored", "scripts/merge/land.go", 0, mergeGlob, ""},
		{"score past the band", "scripts/merge/land.go", 11, mergeGlob, ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			row := planOf(t, clickup.Finding{File: testCase.file, Score: testCase.score})

			if got := row["expected_changes"]; got != testCase.glob {
				t.Errorf("expected_changes = %q, want %q", got, testCase.glob)
			}

			if got := row["triage_score_orderindex"]; got != testCase.index {
				t.Errorf("triage_score_orderindex = %q, want %q", got, testCase.index)
			}
		})
	}
}

// TestPlanFieldsNumbersTicketsFromOne ties each row to the `## ` heading the
// filing turn matches it against — off by one and every field lands on the
// wrong task.
func TestPlanFieldsNumbersTicketsFromOne(t *testing.T) {
	t.Parallel()

	queue := []clickup.Finding{
		{File: "a/one.go", Score: 9},
		{File: "b/two.go", Score: 8},
		{File: "c/three.go", Score: 7},
	}

	var rows []map[string]any

	decode(t, clickup.PlanFieldsForTest(queue, stampMillis, stampCommit), &rows)

	for index, row := range rows {
		if got, want := row["ticket"], float64(index+1); got != want {
			t.Errorf("row %d: ticket = %v, want %v", index, got, want)
		}
	}
}

// planOf renders one finding's plan as strings, so an absent key and a zero
// are distinguishable — which is the whole point of omitting the index.
func planOf(t *testing.T, finding clickup.Finding) map[string]string {
	t.Helper()

	var rows []map[string]any

	decode(t, clickup.PlanFieldsForTest([]clickup.Finding{finding}, stampMillis, stampCommit), &rows)

	if len(rows) != 1 {
		t.Fatalf("planFields returned %d rows, want 1", len(rows))
	}

	flat := map[string]string{}

	for key, value := range rows[0] {
		if number, ok := value.(float64); ok {
			flat[key] = strconv.FormatFloat(number, 'f', -1, 64)

			continue
		}

		text, _ := value.(string)
		flat[key] = text
	}

	return flat
}

func decode(t *testing.T, payload []byte, into any) {
	t.Helper()

	err := json.Unmarshal(payload, into)
	if err != nil {
		t.Fatalf("decoding the plan: %v", err)
	}
}
