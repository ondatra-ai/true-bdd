package clickup_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"github.com/ondatra-ai/true-bdd/scripts/triage"
)

// A hand-written deferral. `## ` opens a ticket and everything under it is raw
// prose — including a `### `, which ticket.yaml now renders around rather than
// taking as the ticket's own shape.
const deferral = `# Deferrals

## Fix the orphaned joins cross-reference

The renumbering moved them.

` + "```" + `
## this is a fence, and splitSections opens a ticket on it anyway
` + "```" + `

## Publish a killable pid

` + "`kill -9`" + ` on ` + "`go run`" + ` never reaches the child.
`

// The two scores the table needs: one that files, one that does not.
const (
	passing = 8
	failing = 3
)

// A deferral reaches the one creator as a queue of candidates. Its prose is
// Body — raw material for `### What to change` — and never the ticket's shape.
func TestFindingsOfCarriesProseNotShape(t *testing.T) {
	t.Parallel()

	queue := clickup.FindingsOfForTest(deferral)

	titles := clickup.TitlesForTest(queue)
	want := []string{
		"Fix the orphaned joins cross-reference",
		"this is a fence, and splitSections opens a ticket on it anyway",
		"Publish a killable pid",
	}

	if len(titles) != len(want) {
		t.Fatalf("split into %d ticket(s): %q, want %q", len(titles), titles, want)
	}

	for index := range want {
		if titles[index] != want[index] {
			t.Errorf("ticket %d is %q, want %q", index, titles[index], want[index])
		}
	}

	if queue[0].Source != "deferral" {
		t.Errorf("source = %q, want the deferral source that names its raiser", queue[0].Source)
	}

	if strings.Contains(queue[0].Body, "## ") {
		t.Errorf("the `## ` line leaked into the body:\n%s", queue[0].Body)
	}
}

// A candidate below the floor is not filed, whatever raised it.
func TestScoredDropsBelowTheFloor(t *testing.T) {
	t.Parallel()

	queue := clickup.FindingsOfForTest(deferral)

	if got := clickup.ScoredForTest(queue, keeping(failing)); len(got) != 0 {
		t.Errorf("kept %d candidate(s) scored %d, want none below the floor", len(got), failing)
	}

	kept := clickup.ScoredForTest(queue, keeping(passing))
	if len(kept) != len(queue) {
		t.Fatalf("kept %d of %d scored %d", len(kept), len(queue), passing)
	}

	if kept[0].Story != anyStory {
		t.Errorf("story = %q, want the verdict's — a creating path always carries one", kept[0].Story)
	}
}

// A candidate the scorer could not judge is dropped, not filed unscored, which
// would leave it unsortable by the queue task-loop works.
func TestScoredDropsWhatItCannotScore(t *testing.T) {
	t.Parallel()

	refused := func(_ triage.Subject) (triage.Verdict, error) {
		return triage.Verdict{}, errRefused
	}

	if got := clickup.ScoredForTest(clickup.FindingsOfForTest(deferral), refused); len(got) != 0 {
		t.Errorf("kept %d unscored candidate(s), want none", len(got))
	}
}

// A row merge already scored keeps its verdict: its disposition was decided
// against merge's Floors table, and re-scoring it here would overrule that.
func TestScoredLeavesAScoredRowAlone(t *testing.T) {
	t.Parallel()

	scored := []clickup.Finding{{Title: "already judged", Score: 9, Reason: "merge said so"}}

	kept := clickup.ScoredForTest(scored, func(_ triage.Subject) (triage.Verdict, error) {
		t.Error("a scored row was sent to the scoring turn")

		return triage.Verdict{}, errRefused
	})

	if len(kept) != 1 || kept[0].Score != 9 {
		t.Errorf("kept %+v, want the row unchanged at 9", kept)
	}
}

// The requirement this change exists for: a deferral is shaped by ticket.yaml,
// not by the prose someone typed. Same four headings, same story, same
// renderer a review finding gets — the source decides only its provenance.
func TestADeferralRendersThroughTheOneTemplate(t *testing.T) {
	t.Parallel()

	kept := clickup.ScoredForTest(clickup.FindingsOfForTest(deferral), keeping(passing))

	document := clickup.Render(kept, "deferred", clickup.DeferralOriginForTest())

	for _, want := range append(headingLines(),
		"A person raised this on a hand-written deferral; triage scored it **8/10**.",
		anyStory,
	) {
		if !strings.Contains(document, want) {
			t.Errorf("a filed deferral does not carry %q:\n%s", want, document)
		}
	}
}

// headingLines is the four `### ` sections, as ticket.yaml declares them.
func headingLines() []string {
	names := clickup.Headings()

	lines := make([]string, 0, len(names))
	for _, name := range names {
		lines = append(lines, "### "+name)
	}

	return lines
}

var errRefused = errors.New("the turn failed")

// keeping is a scorer that answers the same way for every candidate.
func keeping(score int) func(triage.Subject) (triage.Verdict, error) {
	return func(_ triage.Subject) (triage.Verdict, error) {
		return triage.Verdict{Score: score, Reason: anyReason, Story: anyStory}, nil
	}
}
