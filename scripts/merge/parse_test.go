package merge_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/merge"
)

// golden is what the Python this ports produced from the same fixtures.
type golden struct {
	BodyFindings   []merge.ParsedFinding `json:"body_findings"`
	ClaimedCounts  map[string]int        `json:"claimed_counts"`
	ThreadClean    string                `json:"thread_clean"`
	ThreadSeverity string                `json:"thread_severity"`
	ThreadTitle    string                `json:"thread_title"`
}

// TestParsesLikeThePython pins the comment machinery against the script it
// replaces.
//
// This is the part of the merge loop with no other check on it: a helper that
// queried reviewThreads alone saw 16 of 28 findings on PR #70, and nothing
// failed — the run simply missed twelve. The fixture carries every branch that
// mattered there: three-deep nesting, denylisted machinery blocks, the
// cr-comment markers a finding ends at, the footer after the last marker that
// is not a finding, and a severity label whose emoji sits inside the
// underscores.
func TestParsesLikeThePython(t *testing.T) {
	t.Parallel()

	want := readGolden(t)
	reviewBody := readFixture(t, "review-body.md")
	threadBody := readFixture(t, "thread-body.md")

	t.Run("body-only findings", func(t *testing.T) {
		t.Parallel()

		got := merge.ParseReviewBody(reviewBody)
		if len(got) != len(want.BodyFindings) {
			t.Fatalf("extracted %d finding(s), the Python extracted %d", len(got), len(want.BodyFindings))
		}

		for index, finding := range got {
			if !reflect.DeepEqual(finding, want.BodyFindings[index]) {
				t.Errorf("finding %d differs:\n got: %+v\nwant: %+v",
					index, finding, want.BodyFindings[index])
			}
		}
	})

	t.Run("claimed counts", func(t *testing.T) {
		t.Parallel()

		if got := merge.ClaimedCounts(reviewBody); !reflect.DeepEqual(got, want.ClaimedCounts) {
			t.Errorf("claimed counts = %v, want %v", got, want.ClaimedCounts)
		}
	})

	t.Run("thread body", func(t *testing.T) {
		t.Parallel()

		cleaned := merge.Clean(threadBody)
		if cleaned != want.ThreadClean {
			t.Errorf("clean differs:\n got: %q\nwant: %q", cleaned, want.ThreadClean)
		}

		if got := merge.SeverityOf(cleaned); got != want.ThreadSeverity {
			t.Errorf("severity = %q, want %q", got, want.ThreadSeverity)
		}

		if got := merge.TitleOf(cleaned); got != want.ThreadTitle {
			t.Errorf("title = %q, want %q", got, want.ThreadTitle)
		}
	})
}

func readGolden(t *testing.T) golden {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("testdata", "parse.golden.json"))
	if err != nil {
		t.Fatalf("reading the golden: %v", err)
	}

	var want golden

	err = json.Unmarshal(raw, &want)
	if err != nil {
		t.Fatalf("parsing the golden: %v", err)
	}

	return want
}

func readFixture(t *testing.T, name string) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}

	return string(raw)
}
