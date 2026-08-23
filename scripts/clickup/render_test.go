package clickup_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
)

// TestRenderMatchesGolden pins the ticket document byte for byte.
//
// The goldens were produced by the Python `clickup.py render` this replaces,
// so the first run of this test was the port's parity check. They stay as a
// regression pin: the document is what a person reads before anything is
// uploaded and what the model is told to transcribe verbatim, so a stray
// space in it is a change to both.
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
			tag:    "merge-improvements",
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

			if got := clickup.Render(queue, testCase.tag, testCase.pr); got != string(want) {
				t.Errorf("render differs from %s\n%s", testCase.golden, firstDifference(got, string(want)))
			}
		})
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
