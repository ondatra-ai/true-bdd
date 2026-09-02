package triage_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/triage"
)

// Filed is the one branch, and the two clauses are the two homes a story can
// have. A creating path asked to refresh would fail for declining to rewrite a
// body that does not exist; a filed one asked for a story would grow each sweep.
func TestPromptCarriesOneClausePerBranch(t *testing.T) {
	t.Parallel()

	const (
		refreshMarker = "ALSO return `description`"
		storyMarker   = "ALSO return `story`"
	)

	subject := triage.Subject{ID: "86cbb1ceb", Title: "a title", Body: "a body"}
	scoring := triage.PromptForTest(subject)

	if strings.Contains(scoring, refreshMarker) {
		t.Error("a subject with Refresh unset was asked for a refreshed description")
	}

	if !strings.Contains(scoring, storyMarker) {
		t.Error("a subject with Refresh unset was not asked for a story")
	}

	subject.Filed = true
	refreshing := triage.PromptForTest(subject)

	if !strings.Contains(refreshing, refreshMarker) {
		t.Error("a subject with Refresh set was not asked for a refreshed description")
	}

	if strings.Contains(refreshing, storyMarker) {
		t.Error("a refresh was asked for a story as its own field, which would backfill the ticket")
	}
}

// Every prompt carries the rubric, whichever caller built the subject.
func TestPromptAlwaysCarriesTheRubric(t *testing.T) {
	t.Parallel()

	prompt := triage.PromptForTest(triage.Subject{ID: "x"})
	for _, want := range []string{
		"CONSEQUENCE IF LEFT UNDONE",
		"no longer describes this repository",
		"Read the code before you answer",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt does not carry %q", want)
		}
	}
}

// An absent field renders as `?` rather than a dangling label, and the body is
// the last thing in the block so a long one cannot swallow a label after it.
func TestPromptResolvesAbsentFields(t *testing.T) {
	t.Parallel()

	prompt := triage.PromptForTest(triage.Subject{ID: "x", Body: "the body"})

	for _, want := range []string{"origin   : ?", "file     : ?:?", "title    : ?"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt does not carry %q", want)
		}
	}

	if !strings.Contains(prompt, "the body\n--- END SUBJECT ---") {
		t.Error("the body is not the last thing in the subject block")
	}
}

// A refresh rewrites what it was SHOWN and scripts/clickup replaces the
// description with the answer, so a ticket cut short here is one whose tail the
// sweep deletes. Whole means two 4,000-rune fields plus the four headings.
func TestPromptCarriesAWholeTicket(t *testing.T) {
	t.Parallel()

	const wholeTicket = 2*4000 + 700

	body := strings.Repeat("x", wholeTicket) + "\n### Context\n\nthe last heading"

	prompt := triage.PromptForTest(triage.Subject{ID: "x", Body: body, Filed: true})
	if !strings.Contains(prompt, "the last heading") {
		t.Errorf("a %d-rune ticket lost its tail before the turn saw it", len(body))
	}
}
