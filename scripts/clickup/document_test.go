package clickup_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"github.com/ondatra-ai/true-bdd/scripts/triage"
)

// A hand-written deferral: `## ` opens a ticket, `### ` is one of its four
// headings, and neither the title nor a body line may be miscounted.
const deferral = `# Deferrals

## Fix the orphaned joins cross-reference

### Why

The renumbering moved them.

### What to change

` + "```" + `
### this is a fence, not a heading
` + "```" + `

## Publish a killable pid

### Why

` + "`kill -9`" + ` on ` + "`go run`" + ` never reaches the child.
`

func TestCountHeadingsCountsTicketsNotSections(t *testing.T) {
	t.Parallel()

	if got := clickup.CountHeadingsForTest(deferral); got != 2 {
		t.Fatalf("counted %d ticket(s), want 2 — `### ` and `# ` are not tickets", got)
	}
}

func TestCountHeadingsOnADocumentWithNoTicket(t *testing.T) {
	t.Parallel()

	if got := clickup.CountHeadingsForTest("# Title\n\nProse, and no ticket.\n"); got != 0 {
		t.Fatalf("counted %d, want 0 so FileDocument refuses", got)
	}
}

// The document turn must carry the backlog stamp the queue turn carries:
// one const feeds both, and this is what proves it reached this one.
func TestDocumentPromptStampsBacklog(t *testing.T) {
	t.Parallel()

	prompt := clickup.DocumentPromptForTest(deferral, "deferred", passing)

	for _, want := range []string{
		"- status: backlog",
		"Never `to do`",
		"- tag: deferred",
		"Exactly one task per `## ` heading — 2 in total.",
		"Expected Changes, Scope and Good For Agent are a\nperson's to fill",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt does not carry %q", want)
		}
	}

	if !strings.Contains(prompt, "Publish a killable pid") {
		t.Error("the document was not embedded verbatim")
	}
}

// The two scores the table needs: one that files, one that does not.
const (
	passing = 8
	failing = 3
)

// splitSections and countHeadings must never disagree about how many tickets
// a document holds — the count check that catches a silent drop compares one
// against the other.
func TestSplitSectionsAgreesWithCountHeadings(t *testing.T) {
	t.Parallel()

	kept := clickup.TriageSectionsForTest(deferral, keeping(passing))

	if got, want := len(kept), clickup.CountHeadingsForTest(deferral); got != want {
		t.Fatalf("split into %d section(s), countHeadings says %d", got, want)
	}

	titles := clickup.SectionTitlesForTest(kept)
	if titles[0] != "Fix the orphaned joins cross-reference" || titles[1] != "Publish a killable pid" {
		t.Errorf("titles = %q, want the two `## ` headings in order", titles)
	}
}

// A section below the floor is not filed. It leaves the document, and the
// count the turn is given shortens with it.
func TestTriageSectionsDropsBelowTheFloor(t *testing.T) {
	t.Parallel()

	if got := clickup.TriageSectionsForTest(deferral, keeping(failing)); len(got) != 0 {
		t.Errorf("kept %d section(s) scored %d, want none below the floor", len(got), failing)
	}

	prompt := clickup.DocumentPromptForTest(deferral, "deferred", passing)
	if !strings.Contains(prompt, "— 2 in total.") {
		t.Error("the surviving count did not reach the prompt")
	}
}

// A section the scorer could not judge is dropped, not filed unscored — which
// is what this path did before, leaving every deferral unsortable by task-loop.
func TestTriageSectionsDropsWhatItCannotScore(t *testing.T) {
	t.Parallel()

	refused := func(_ triage.Subject) (triage.Verdict, error) {
		return triage.Verdict{}, errRefused
	}

	if got := clickup.TriageSectionsForTest(deferral, refused); len(got) != 0 {
		t.Errorf("kept %d unscored section(s), want none", len(got))
	}
}

var errRefused = errors.New("the turn failed")

// keeping is a scorer that answers the same way for every section.
func keeping(score int) func(triage.Subject) (triage.Verdict, error) {
	return func(_ triage.Subject) (triage.Verdict, error) {
		return triage.Verdict{
			Score: score, Reason: anyReason, Description: anyRefreshed,
		}, nil
	}
}
