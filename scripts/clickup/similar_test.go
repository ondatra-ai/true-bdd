package clickup_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
)

// audited is the ticket a `dupes` run is judging, and so cannot match.
const audited = "86self"

// corpus is the tracker every validation case is judged against.
func corpus() []clickup.CorpusRow {
	return []clickup.CorpusRow{
		{ID: older, Name: "Tee the run's own stdout", Status: backlog, URL: olderURL},
		{ID: audited, Name: "The audited ticket", Status: backlog, URL: "u/" + audited},
	}
}

func good() clickup.Match {
	return clickup.Match{ID: older, Score: 9, Reason: "both tee the same stream"}
}

// --json-schema is an instruction to a model, so the band, the row count and
// every id are checked again on the way back in.
func TestResolveRejectsWhatTheSchemaCannotCatch(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		matches []clickup.Match
		exclude string
	}{
		"a fourth match": {
			matches: []clickup.Match{good(), good(), good(), good()},
		},
		"a score below the band": {
			matches: []clickup.Match{{ID: older, Score: 0, Reason: anyReason}},
		},
		"a score above the band": {
			matches: []clickup.Match{{ID: older, Score: 11, Reason: anyReason}},
		},
		"no reason": {
			matches: []clickup.Match{{ID: older, Score: 9, Reason: "  "}},
		},
		"an id the corpus does not hold": {
			matches: []clickup.Match{{ID: "86invented", Score: 9, Reason: anyReason}},
		},
		"the audited ticket itself": {
			matches: []clickup.Match{{ID: audited, Score: 10, Reason: "it is itself"}},
			exclude: audited,
		},
	}

	for name, test := range tests {
		_, err := clickup.ResolveMatchesForTest(corpus(), test.matches, test.exclude)
		if err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// Nothing resembling the candidate is an answer, not a failure: it is what
// every first filing of a new proposal looks like.
func TestResolveAcceptsAnEmptyAnswer(t *testing.T) {
	t.Parallel()

	matches, err := clickup.ResolveMatchesForTest(corpus(), nil, "")
	if err != nil {
		t.Fatalf("an empty answer was rejected: %v", err)
	}

	if len(matches) != 0 {
		t.Fatalf("resolved %d match(es) from an empty answer", len(matches))
	}
}

// The turn is asked for an id and a score; the url and the status are read off
// the corpus, because asking for a fact already on disk invites a wrong one.
func TestResolveFillsTheMatchFromTheCorpusNotTheAnswer(t *testing.T) {
	t.Parallel()

	matches, err := clickup.ResolveMatchesForTest(corpus(), []clickup.Match{good()}, "")
	if err != nil {
		t.Fatalf("resolving: %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("resolved %d match(es), want 1", len(matches))
	}

	if matches[0].URL != "u/86old" || matches[0].Status != backlog {
		t.Errorf("match resolved to url %q status %q, want the corpus's",
			matches[0].URL, matches[0].Status)
	}
}
