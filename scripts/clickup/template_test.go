package clickup_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
)

// The four headings CLAUDE.md requires of a deferral, in order. Restated here
// so reordering or renaming one in ticket.yaml fails rather than quietly
// changing the shape of every ticket this repository files.
func TestTicketHeadingsAreTheFourInOrder(t *testing.T) {
	t.Parallel()

	got := clickup.HeadingNamesForTest()
	want := []string{"Why", "What to change", "Verification", "Context"}

	if len(got) != len(want) {
		t.Fatalf("ticket.yaml declares %d heading(s): %q, want %q", len(got), got, want)
	}

	for index := range want {
		if got[index] != want[index] {
			t.Errorf("heading %d is %q, want %q", index, got[index], want[index])
		}
	}
}

func TestTicketStatusIsBacklog(t *testing.T) {
	t.Parallel()

	if got := clickup.TicketStatusForTest(); got != backlog {
		t.Fatalf("ticket.yaml files to %q — `to do` is the queue task-loop works unattended", got)
	}
}

// The rule is prose and the status is a value; nothing but this makes them
// agree, so editing one without the other has to fail here.
func TestStatusRuleNamesTheStatusItDeclares(t *testing.T) {
	t.Parallel()

	rule, status := clickup.StatusRuleForTest(), clickup.TicketStatusForTest()

	if !strings.Contains(rule, "`"+status+"`") {
		t.Fatalf("status_rule %q does not name the status %q it is meant to state", rule, status)
	}
}

// TestRaiserNamesWhoActuallyRaisedIt pins the mapping that replaced a hardcoded
// "CodeRabbit raised this". Three paths file through one template, and for 35
// tickets it credited CodeRabbit with all three.
func TestRaiserNamesWhoActuallyRaisedIt(t *testing.T) {
	t.Parallel()

	// The two values scripts/merge/comments.go sets, and the postmortem's.
	cases := map[string]string{
		"thread":     "CodeRabbit",
		"body-only":  "CodeRabbit",
		"postmortem": "The merge postmortem",
		"":           "An unrecorded source",
	}

	for source, want := range cases {
		if got := clickup.RaiserForTest(source); got != want {
			t.Errorf("raiserOf(%q) = %q, want %q", source, got, want)
		}
	}
}
