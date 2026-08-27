package clickup_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
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

	prompt := clickup.DocumentPromptForTest(deferral, "deferred")

	for _, want := range []string{
		"- status: backlog",
		"Never `to do`",
		"- tag: deferred",
		"Exactly one task per `## ` heading — 2 in total.",
		"Set no custom field.",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt does not carry %q", want)
		}
	}

	if !strings.Contains(prompt, "Publish a killable pid") {
		t.Error("the document was not embedded verbatim")
	}
}
