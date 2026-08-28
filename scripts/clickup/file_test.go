package clickup_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
)

// matchWidth is the window dropAlreadyOpen compares through, restated here so
// the boundary case fails if it ever moves.
const matchWidth = 60

// backlog is the status ticket.yaml declares; the tests assert against the
// literal rather than the accessor, so a change there has to be deliberate.
const backlog = "backlog"

// The two ticket ids both the dedupe filter and the backlog check reuse.
const (
	openWindowID = "86cb9fedx"
	openStateID  = "86cb9feh1"
)

// Two proposals PR #89's postmortem filed, and one PR #90's proposed fresh.
const (
	windowTitle = "Filter the history extract to this run's own window, not the whole file"
	stateTitle  = "Clear tmp/merge/ at the start of every run"
	skillTitle  = "Rewrite the pr-merge SKILL.md"
)

// TestDropAlreadyOpenKeepsWhatIsNotOpen pins the filter the postmortem's path
// runs: a proposal an open ticket already covers is dropped and named, and
// everything else reaches the render untouched.
func TestDropAlreadyOpenKeepsWhatIsNotOpen(t *testing.T) {
	t.Parallel()

	queue := []clickup.Finding{{Title: windowTitle}, {Title: skillTitle}, {Title: stateTitle}}
	open := []clickup.Task{
		{ID: openWindowID, Name: windowTitle},
		{ID: openStateID, Name: stateTitle},
	}

	keptFindings, dropped := clickup.DropAlreadyOpenForTest(queue, open)

	kept := titlesOf(keptFindings)
	if len(kept) != 1 || kept[0] != skillTitle {
		t.Fatalf("kept %q, want only %q", kept, skillTitle)
	}

	named := make([]string, 0, len(dropped))
	for _, task := range dropped {
		named = append(named, task.ID)
	}

	if len(named) != 2 || !slices.Contains(named, openWindowID) || !slices.Contains(named, openStateID) {
		t.Errorf("dropped %q, want both open tickets named", named)
	}
}

// TestDropAlreadyOpenComparesOnlyTheWindow ties the comparison to the one
// ticketURL pairs a finding with its ticket by (scripts/merge/resolve.go:30):
// a tail past the window cannot split a pair, the last rune inside it can.
func TestDropAlreadyOpenComparesOnlyTheWindow(t *testing.T) {
	t.Parallel()

	window := string([]rune(windowTitle)[:matchWidth])

	cases := []struct {
		name string
		open string
		kept int
	}{
		{"differing only past the window", window + " — filed on PR #89", 0},
		{"differing at the last rune inside it", window[:matchWidth-1] + "X" + " — filed on PR #89", 1},
		{"sharing no prefix at all", stateTitle, 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			kept, _ := clickup.DropAlreadyOpenForTest(
				[]clickup.Finding{{Title: window + ", proposed again"}},
				[]clickup.Task{{ID: "86cb9fu0g", Name: testCase.open}})

			if len(kept) != testCase.kept {
				t.Errorf("kept %d finding(s), want %d", len(kept), testCase.kept)
			}
		})
	}
}

// TestDropAlreadyOpenWithNothingOpen is the first postmortem under a tag: an
// empty listing must file the whole queue, not swallow it.
func TestDropAlreadyOpenWithNothingOpen(t *testing.T) {
	t.Parallel()

	queue := []clickup.Finding{{Title: windowTitle}, {Title: stateTitle}}

	kept, dropped := clickup.DropAlreadyOpenForTest(queue, nil)

	if len(kept) != len(queue) {
		t.Errorf("kept %d finding(s), want all %d", len(kept), len(queue))
	}

	if len(dropped) != 0 {
		t.Errorf("dropped %v, want nothing dropped", dropped)
	}
}

func titlesOf(findings []clickup.Finding) []string {
	titles := make([]string, 0, len(findings))

	for _, finding := range findings {
		titles = append(titles, finding.Title)
	}

	return titles
}

// TestWarnMisplacedNamesEveryTicketOutsideBacklog pins the check that stands
// between a misfiled proposal and an unattended task-loop picking it up: the
// filing turn stamps `backlog`, and anything else has to be seen.
func TestWarnMisplacedNamesEveryTicketOutsideBacklog(t *testing.T) {
	t.Parallel()

	named := strings.Join(clickup.MisplacedForTest([]clickup.Ticket{
		{ID: "86cbaymyq", Status: "to do"},
		{ID: openWindowID, Status: backlog},
		{ID: openStateID, Status: "BACKLOG"},
		{ID: "86cb9fej4", Status: ""},
	}), ", ")

	for _, want := range []string{"86cbaymyq (to do)", "86cb9fej4 (?)"} {
		if !strings.Contains(named, want) {
			t.Errorf("%q does not name %q", named, want)
		}
	}

	// Case is the list's presentation, not a second status.
	for _, unwanted := range []string{openWindowID, openStateID} {
		if strings.Contains(named, unwanted) {
			t.Errorf("%q names %q, which is in the backlog", named, unwanted)
		}
	}
}

func TestWarnMisplacedIsSilentWhenEverythingIsFiled(t *testing.T) {
	t.Parallel()

	named := clickup.MisplacedForTest([]clickup.Ticket{{ID: openWindowID, Status: backlog}})

	if len(named) != 0 {
		t.Fatalf("named %v, want silence", named)
	}
}
