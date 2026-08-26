package clickup_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
)

// matchWidth is the window dropAlreadyOpen compares through, restated here so
// the boundary case fails if it ever moves.
const matchWidth = 60

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
		{ID: "86cb9fedx", Name: windowTitle},
		{ID: "86cb9feh1", Name: stateTitle},
	}

	var out strings.Builder

	kept := titlesOf(clickup.DropAlreadyOpenForTest(&out, queue, open))

	if len(kept) != 1 || kept[0] != skillTitle {
		t.Fatalf("kept %q, want only %q", kept, skillTitle)
	}

	for _, want := range []string{"already open 86cb9fedx", "already open 86cb9feh1"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output %q does not name the drop %q", out.String(), want)
		}
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

			var out strings.Builder

			kept := clickup.DropAlreadyOpenForTest(&out,
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

	var out strings.Builder

	kept := clickup.DropAlreadyOpenForTest(&out, queue, nil)

	if len(kept) != len(queue) {
		t.Errorf("kept %d finding(s), want all %d", len(kept), len(queue))
	}

	if out.String() != "" {
		t.Errorf("output = %q, want nothing dropped", out.String())
	}
}

func titlesOf(findings []clickup.Finding) []string {
	titles := make([]string, 0, len(findings))

	for _, finding := range findings {
		titles = append(titles, finding.Title)
	}

	return titles
}
