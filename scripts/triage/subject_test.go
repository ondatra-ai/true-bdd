package triage_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/triage"
)

// The refresh clause is the one branch in a shared function. A merge finding
// that carried it would be failed for declining to rewrite a body with no
// headings; a ticket that lost it would never be refreshed at all.
func TestPromptCarriesTheRefreshClauseOnlyUnderRefresh(t *testing.T) {
	t.Parallel()

	const marker = "ALSO return `description`"

	subject := triage.Subject{ID: "86cbb1ceb", Title: "a title", Body: "a body"}

	if strings.Contains(triage.PromptForTest(subject), marker) {
		t.Error("a subject with Refresh unset was asked for a refreshed description")
	}

	subject.Refresh = true

	if !strings.Contains(triage.PromptForTest(subject), marker) {
		t.Error("a subject with Refresh set was not asked for a refreshed description")
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
