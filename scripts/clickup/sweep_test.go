package clickup_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"github.com/ondatra-ai/true-bdd/scripts/triage"
)

// The other two statuses the sweep names, restated so the test fails if
// either is renamed out from under it. `backlog` is file_test.go's.
const (
	queued      = "to do"
	notRelevant = "not relevant"
)

// TestSelectStaleTakesTheOldestTriagedFirst pins the ordering the whole sweep
// rests on. Get it wrong and every run re-walks the same tickets, because the
// stamp it just wrote is what should have moved them to the back.
func TestSelectStaleTakesTheOldestTriagedFirst(t *testing.T) {
	t.Parallel()

	listed := []clickup.Task{
		{ID: "recent", TriageDate: "1787968800000", Created: anyCreated},
		{ID: "never-b", TriageDate: "", Created: "2026-02-02T00:00:00Z"},
		{ID: "old", TriageDate: "1772323200000", Created: anyCreated},
		{ID: "never-a", TriageDate: "", Created: anyCreated},
	}

	got := ids(clickup.SelectStaleForTest(listed, 4))

	// Never-triaged first, oldest created breaking their tie; then by stamp.
	want := "never-a never-b old recent"
	if strings.Join(got, " ") != want {
		t.Errorf("selectStale ordered %q, want %q", strings.Join(got, " "), want)
	}
}

func TestSelectStaleCapsAtTheCount(t *testing.T) {
	t.Parallel()

	listed := []clickup.Task{{ID: "a"}, {ID: "b"}, {ID: "c"}}

	cases := map[int]int{1: 1, 3: 3, 9: 3}
	for count, want := range cases {
		if got := len(clickup.SelectStaleForTest(listed, count)); got != want {
			t.Errorf("selectStale(%d tickets, count %d) took %d, want %d",
				len(listed), count, got, want)
		}
	}
}

// The sweep never leaves the input slice reordered under its caller.
func TestSelectStaleDoesNotReorderItsInput(t *testing.T) {
	t.Parallel()

	listed := []clickup.Task{
		{ID: "b", TriageDate: "1785535200000"},
		{ID: "a", TriageDate: ""},
	}

	clickup.SelectStaleForTest(listed, 2)

	if listed[0].ID != "b" {
		t.Errorf("the caller's slice was reordered: %q is first", listed[0].ID)
	}
}

// TestDispositionRetiresBelowTheFloorAndTouchesNothingAbove pins the one
// decision the sweep makes about a status. Above the floor the status is left
// exactly where a person put it — promoting or demoting is not a sweep's job.
func TestDispositionRetiresBelowTheFloorAndTouchesNothingAbove(t *testing.T) {
	t.Parallel()

	cases := []struct {
		score int
		was   string
		want  string
	}{
		{1, backlog, notRelevant},
		{5, backlog, notRelevant},
		{5, queued, notRelevant},
		{6, backlog, backlog},
		{6, queued, queued},
		{10, queued, queued},
	}

	for _, test := range cases {
		verdict := triage.Verdict{Score: test.score, Reason: anyReason}
		if got := clickup.DispositionForTest(verdict, test.was); got != test.want {
			t.Errorf("a %d scored in %q became %q, want %q",
				test.score, test.was, got, test.want)
		}
	}
}

// The listing turn must page. A page holds 100 and this list has more than
// one, so a turn that stops at the first silently loses the rest.
func TestWalkPromptPagesAndNamesBothStatuses(t *testing.T) {
	t.Parallel()

	prompt := clickup.WalkPromptForTest()
	for _, want := range []string{
		"until a page comes back empty",
		"a raw millisecond number, copied digit for digit",
		`"` + backlog + `" or "` + queued + `"`,
		"Do not sort, filter, deduplicate or omit a row",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the walk prompt does not carry %q", want)
		}
	}
}

// The dropdown is addressed by POSITION, and the date field takes unix
// milliseconds. Both are the kind of mistake that writes a plausible wrong
// value rather than failing.
func TestApplyPromptPassesTheIndexAndTheMilliseconds(t *testing.T) {
	t.Parallel()

	ticket := clickup.Task{ID: "86cbb1ceb", Status: backlog}
	verdict := triage.Verdict{Score: 7, Reason: anyReason, Description: anyRefreshed}

	prompt := clickup.ApplyPromptForTest(ticket, verdict, 1787999823623, "eeb67aa")

	for _, want := range []string{
		"update task 86cbb1ceb and nothing else",
		`Set its status to "backlog"`,
		"the integer 6.",
		"the integer 1787999823623.",
		`the string "eeb67aa"`,
		"Replace its description",
		anyRefreshed,
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the apply prompt does not carry %q", want)
		}
	}
}

// The turn answers under a schema, never in prose. A prose answer was parsed
// here once and "All four writes succeeded.\n\nOK" read as a refusal, because
// the OK was not the prefix — every write had landed.
func TestApplyAnswersUnderASchema(t *testing.T) {
	t.Parallel()

	prompt := clickup.ApplyPromptForTest(
		clickup.Task{ID: "x", Status: backlog}, triage.Verdict{Score: 7, Reason: anyReason}, 1, "abc")

	if strings.Contains(prompt, "reply with one word") {
		t.Error("the apply turn still asks for a prose answer")
	}

	if !strings.Contains(clickup.ApplySchemaForTest(), `"required":["ok","error"]`) {
		t.Error("the apply schema does not require both fields")
	}
}

// No refresh means no description step: a turn told to "replace" with nothing
// would blank the body it was asked to leave alone.
func TestApplyPromptLeavesTheBodyAloneWithoutARefresh(t *testing.T) {
	t.Parallel()

	ticket := clickup.Task{ID: "86cbb1ceb", Status: backlog}
	verdict := triage.Verdict{Score: 3, Reason: "the file is gone"}

	prompt := clickup.ApplyPromptForTest(ticket, verdict, 1, "abc")

	if !strings.Contains(prompt, "Leave its description exactly as it is") {
		t.Error("a verdict with no refresh did not leave the description alone")
	}

	if strings.Contains(prompt, "BEGIN DESCRIPTION") {
		t.Error("an empty description reached the prompt as a replacement")
	}

	if !strings.Contains(prompt, `Set its status to "`+notRelevant+`"`) {
		t.Error("a verdict below the floor did not retire the ticket")
	}
}

func ids(tasks []clickup.Task) []string {
	got := make([]string, 0, len(tasks))
	for _, task := range tasks {
		got = append(got, task.ID)
	}

	return got
}
