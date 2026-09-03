package clickup_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"github.com/ondatra-ai/true-bdd/scripts/triage"
)

// The two closed statuses the fence refuses, restated so a rename fails here.
const (
	done   = "done"
	failed = "failed"
)

// Both write turns must name the PLAIN `description` parameter. Observed
// 2026-09-03: a body written through `markdownContent` comes back from getTask
// flattened, every `### ` gone, which broke all 42 tickets filed before this.
func TestBothWriteTurnsForbidMarkdownContent(t *testing.T) {
	t.Parallel()

	refreshed := triage.Verdict{Score: 7, Reason: "still live", Description: "### Why\n\nx"}

	prompts := map[string]string{
		"the creating turn":  clickup.FilePromptTemplateForTest(),
		"the refreshing one": clickup.ApplyPromptForTest(clickup.Task{ID: "x"}, refreshed, 1, "sha"),
	}

	for name, prompt := range prompts {
		if !strings.Contains(prompt, "never markdownContent") &&
			!strings.Contains(prompt, "NEVER as `markdownContent`") {
			t.Errorf("%s does not forbid markdownContent", name)
		}

		if !strings.Contains(prompt, "description") {
			t.Errorf("%s does not name the plain description parameter", name)
		}
	}
}

// A person pastes whichever of the two ClickUp shows them.
func TestTicketIDReadsAnIDOrAURL(t *testing.T) {
	t.Parallel()

	const want = "86cb9feh1"

	for _, ref := range []string{
		want,
		"  " + want + "  ",
		"https://app.clickup.com/t/" + want,
		"https://app.clickup.com/t/90151491867/" + want,
		"https://app.clickup.com/t/90151491867/" + want + "/",
	} {
		if got := clickup.TicketIDForTest(ref); got != want {
			t.Errorf("ticketID(%q) = %q, want %q", ref, got, want)
		}
	}
}

// The per-ticket form must not reach where the count form never could:
// dispositionOf retires anything below the floor, and retiring a `done` ticket
// rewrites work somebody already finished.
func TestWalkableFencesTheStatusesTheSweepWalks(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		backlog:     true,
		queued:      true,
		"TO DO":     true,
		notRelevant: false,
		done:        false,
		failed:      false,
		"":          false,
	}

	for status, want := range cases {
		if got := clickup.WalkableForTest(status) == nil; got != want {
			t.Errorf("walkable(%q) accepted = %v, want %v", status, got, want)
		}
	}
}
