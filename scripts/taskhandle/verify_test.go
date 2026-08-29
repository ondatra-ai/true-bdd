package taskhandle_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"github.com/ondatra-ai/true-bdd/scripts/taskhandle"
)

const goodSHA = "6fa40cd0000000000000000000000000000000aa"

func ready() taskhandle.Detail {
	sections := make([]string, 0, 4*len(clickup.Headings()))

	for _, heading := range clickup.Headings() {
		sections = append(sections, "### "+heading, "", "something", "")
	}

	body := strings.Join(sections, "\n")

	return taskhandle.Detail{
		ID: "abc", Name: "a ticket", Status: "TO DO",
		Description: body, TriageScore: "7", TriageDate: "1788002290647",
		TriageCommit: goodSHA, ExpectedChanges: "scripts/**", GoodForAgent: true,
	}
}

func TestVerifyPassesAReadyTicket(t *testing.T) {
	t.Parallel()

	if missing := taskhandle.Verify(ready(), clickup.Headings()); len(missing) > 0 {
		t.Errorf("a ready Ticket was refused: %v", missing)
	}
}

func TestVerifyNamesEachMissingField(t *testing.T) {
	t.Parallel()

	for name, breakIt := range map[string]func(*taskhandle.Detail){
		"Good For Agent":   func(d *taskhandle.Detail) { d.GoodForAgent = false },
		"Triage Score":     func(d *taskhandle.Detail) { d.TriageScore = "" },
		"Triage Date":      func(d *taskhandle.Detail) { d.TriageDate = "  " },
		"Triage Commit":    func(d *taskhandle.Detail) { d.TriageCommit = "6fa40cd" },
		"Expected Changes": func(d *taskhandle.Detail) { d.ExpectedChanges = "" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			detail := ready()
			breakIt(&detail)

			missing := taskhandle.Verify(detail, clickup.Headings())
			if len(missing) != 1 {
				t.Fatalf("got %d complaints, want exactly 1: %v", len(missing), missing)
			}

			if !strings.Contains(missing[0], name) {
				t.Errorf("complaint %q does not name %q", missing[0], name)
			}
		})
	}
}

// An out-of-band score is as unusable as an absent one: it orders the queue.
func TestVerifyRefusesAScoreOutsideTheBand(t *testing.T) {
	t.Parallel()

	for _, score := range []string{"0", "11", "high", "-3"} {
		detail := ready()
		detail.TriageScore = score

		if missing := taskhandle.Verify(detail, clickup.Headings()); len(missing) == 0 {
			t.Errorf("score %q was accepted", score)
		}
	}
}

func TestVerifyNamesEachMissingHeading(t *testing.T) {
	t.Parallel()

	for _, heading := range clickup.Headings() {
		detail := ready()
		detail.Description = strings.ReplaceAll(detail.Description, "### "+heading, "## "+heading)

		missing := taskhandle.Verify(detail, clickup.Headings())
		if len(missing) != 1 || !strings.Contains(missing[0], heading) {
			t.Errorf("dropping %q gave %v, want one complaint naming it", heading, missing)
		}
	}
}

func TestVerifyRefusesAnyStatusButToDo(t *testing.T) {
	t.Parallel()

	for _, status := range []string{"PROCESSING", "DONE", "FAILED", "backlog"} {
		detail := ready()
		detail.Status = status

		if missing := taskhandle.Verify(detail, clickup.Headings()); len(missing) == 0 {
			t.Errorf("status %q was accepted; that Ticket is somebody's work", status)
		}
	}
}
