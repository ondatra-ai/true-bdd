package clickup_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
)

// TestRenderMatchesGolden pins the ticket document byte for byte (the
// goldens came from the Python this replaces). A stray space is a change
// both a person reading it and a model transcribing it would see.
func TestRenderMatchesGolden(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		queue  string
		golden string
		tag    string
		pr     string
	}{
		{
			name:   "edge cases",
			queue:  "queue-edges.json",
			golden: "queue-edges.golden.md",
			tag:    "fix-now",
			pr:     "76",
		},
		{
			// No --pr: the source line reads "a local review" instead.
			name:   "no pull request",
			queue:  "queue-edges.json",
			golden: "queue-edges-nopr.golden.md",
			tag:    "other",
			pr:     "",
		},
		{
			// A real postmortem queue, markdown tables and all.
			name:   "real queue",
			queue:  "queue-real.json",
			golden: "queue-real.golden.md",
			tag:    improve,
			pr:     "81",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			queue, err := clickup.LoadQueue(filepath.Join("testdata", testCase.queue))
			if err != nil {
				t.Fatalf("loading the queue: %v", err)
			}

			want, err := os.ReadFile(filepath.Join("testdata", testCase.golden))
			if err != nil {
				t.Fatalf("reading the golden: %v", err)
			}

			origin := clickup.Origin(testCase.pr)
			if got := clickup.Render(queue, testCase.tag, origin); got != string(want) {
				t.Errorf("render differs from %s\n%s", testCase.golden, firstDifference(got, string(want)))
			}
		})
	}
}

// The story renders under `### Why`, and its absence renders exactly what it
// always did — which is what keeps every ticket already filed out of this
// change.
func TestStoryRendersUnderWhyOrNotAtAll(t *testing.T) {
	t.Parallel()

	bare := clickup.Finding{Title: "a title", Source: "thread", Score: 7, Reason: "the reason"}
	told := bare
	told.Story = "How it behaves today.\n\nWhat goes wrong, and where it is fixed."

	const (
		quoted = "> the reason\n"
		next   = "\n### What to change"
	)

	if located := clickup.Render([]clickup.Finding{{File: "?", Line: "?"}}, "t", "o"); strings.Contains(located, "`?:?`") {
		t.Error("a row whose file is the literal `?` still renders `?:?` as a location")
	}

	without := clickup.Render([]clickup.Finding{bare}, "fix-now", clickup.Origin("76"))
	if !strings.Contains(without, quoted+next) {
		t.Errorf("a finding with no story does not render as it did:\n%s", without)
	}

	with := clickup.Render([]clickup.Finding{told}, "fix-now", clickup.Origin("76"))
	if !strings.Contains(with, quoted+"\n"+told.Story+"\n"+next) {
		t.Errorf("the story is not under `### Why` in its own paragraph:\n%s", with)
	}
}

// firstDifference locates the mismatch, because the documents run to
// thousands of lines and a full dump says nothing.
func firstDifference(got, want string) string {
	limit := min(len(got), len(want))

	for index := range limit {
		if got[index] != want[index] {
			return context(got, want, index)
		}
	}

	if len(got) != len(want) {
		return context(got, want, limit)
	}

	return "(identical)"
}

func context(got, want string, offset int) string {
	const window = 60

	from := max(offset-window, 0)

	return "first difference at byte " + strconv.Itoa(offset) +
		"\n  got : " + quote(got, from, offset+window) +
		"\n  want: " + quote(want, from, offset+window)
}

func quote(text string, from, to int) string {
	return strconv.Quote(text[from:min(to, len(text))])
}
