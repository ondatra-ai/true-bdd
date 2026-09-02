package clickup_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
)

func row(id, status, score, triaged, created string) clickup.CorpusRow {
	return clickup.CorpusRow{
		ID: id, Name: "a proposal", Status: status,
		URL: "u/" + id, TriageScore: score, TriageDate: triaged, Created: created,
	}
}

// The keeper rule, in the order it settles ties. The pair that started this
// is the last case: 86cbd5az9 scored 7 against 86cbcy55q's 6, so the newer
// copy keeps and picking the older one by hand would have been wrong.
func TestKeeperRuleInOrder(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		members []clickup.CorpusRow
		want    string
	}{
		"what shipped beats what is queued": {
			members: []clickup.CorpusRow{
				row("a", queued, "9", "2", "1"),
				row("b", "done", "1", "1", "2"),
			},
			want: "b",
		},
		"a promotion beats the backlog": {
			members: []clickup.CorpusRow{
				row("a", backlog, "9", "2", "1"),
				row("b", queued, "1", "1", "2"),
			},
			want: "b",
		},
		"a retired copy never keeps": {
			members: []clickup.CorpusRow{
				row("a", notRelevant, "10", "9", "1"),
				row("b", backlog, "1", "1", "2"),
			},
			want: "b",
		},
		"the fresher judgement breaks a tied score": {
			members: []clickup.CorpusRow{
				row("a", backlog, "7", "1000", "1"),
				row("b", backlog, "7", "2000", "2"),
			},
			want: "b",
		},
		"the older ticket breaks everything else": {
			members: []clickup.CorpusRow{
				row("a", backlog, "7", "1000", "2026-09-01"),
				row("b", backlog, "7", "1000", "2026-09-02"),
			},
			want: "a",
		},
		"the higher score keeps": {
			members: []clickup.CorpusRow{
				row("86cbcy55q", backlog, "6", "1788314400000", "2026-09-01"),
				row("86cbd5az9", backlog, "7", "1788314400000", "2026-09-02"),
			},
			want: "86cbd5az9",
		},
	}

	for name, test := range tests {
		if got := clickup.KeeperForTest(test.members); got != test.want {
			t.Errorf("%s: kept %q, want %q", name, got, test.want)
		}
	}
}

// A cluster is transitive: five tickets each judged against one other are one
// proposal filed five times, not four separate pairs.
func TestClustersFoldPairsIntoOneGroup(t *testing.T) {
	t.Parallel()

	rows := []clickup.CorpusRow{
		row("a", backlog, "9", "1", "1"),
		row("b", backlog, "6", "1", "2"),
		row("c", backlog, "5", "1", "3"),
		row("y", backlog, "8", "1", "4"),
		row("z", backlog, "7", "1", "5"),
	}

	got := clickup.ClustersForTest(rows, [][2]string{{"a", "b"}, {"b", "c"}, {"y", "z"}})

	if len(got) != 2 {
		t.Fatalf("folded into %d cluster(s): %v, want 2", len(got), got)
	}

	if len(got[0]) != 3 || got[0][0] != "a" {
		t.Errorf("cluster %v, want a,b,c keeping a — the highest score", got[0])
	}

	if len(got[1]) != 2 || got[1][0] != "y" {
		t.Errorf("cluster %v, want y,z keeping y — the highest score", got[1])
	}
}

// The turn reads the corpus off disk, so the entry has to carry the fields the
// keeper rule and the report are built from, and the body whole.
func TestCorpusEntryCarriesTheHeaderAndTheBody(t *testing.T) {
	t.Parallel()

	entry := clickup.CorpusEntryForTest(clickup.CorpusRow{
		ID: older, Name: "Tee the run's stdout", Status: backlog,
		URL: olderURL, Tags: improve, Created: "2026-08-25",
		TriageScore: "7", TriageDate: "1788314400000",
		Description: "### Why\n\nthe transcript is empty.",
	})

	for _, want := range []string{
		"Tee the run's stdout", older, backlog, olderURL,
		improve, "7", "1788314400000", "the transcript is empty.",
	} {
		if !strings.Contains(entry, want) {
			t.Errorf("the corpus entry does not carry %q:\n%s", want, entry)
		}
	}
}
