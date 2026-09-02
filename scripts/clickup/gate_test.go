package clickup_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
)

// The ticket a fake ranker always answers with, and the queue titles.
const (
	older    = "86old"
	olderURL = "u/86old"
	first    = "one"
	third    = "three"
	improve  = "merge-improvements"
)

// never resembles anything: the corpus holds nothing like the candidate.
func never(_ clickup.Finding, _ string) ([]clickup.Match, error) {
	return nil, nil
}

// scoring answers with one match at the given identity, whatever it is asked.
func scoring(score int) func(clickup.Finding, string) ([]clickup.Match, error) {
	return func(_ clickup.Finding, _ string) ([]clickup.Match, error) {
		return []clickup.Match{{ID: older, Score: score, Reason: "the same fix"}}, nil
	}
}

func titles(queue []clickup.Finding) []string {
	names := make([]string, 0, len(queue))
	for _, finding := range queue {
		names = append(names, finding.Title)
	}

	return names
}

// The user's rule, at both sides of the line: file only on 1-3.
func TestGateFilesUpToThreeAndBlocksFromFour(t *testing.T) {
	t.Parallel()

	queue := []clickup.Finding{{Title: "Bound the history extract by stamp"}}

	for score, wantFiled := range map[int]bool{1: true, 3: true, 4: false, 10: false} {
		kept, err := clickup.RunGateForTest(queue, nil, scoring(score))
		if err != nil {
			t.Fatalf("identity %d: %v", score, err)
		}

		if filed := len(kept) == 1; filed != wantFiled {
			t.Errorf("identity %d filed=%v, want %v", score, filed, wantFiled)
		}
	}
}

// The count the render, the field plan and report's check all agree on is the
// SURVIVORS' — so the drop has to happen before any of them see the queue.
func TestGateReturnsOnlyTheSurvivors(t *testing.T) {
	t.Parallel()

	queue := []clickup.Finding{{Title: first}, {Title: "two"}, {Title: third}}

	kept, err := clickup.RunGateForTest(queue, nil, scoring(9))
	if err != nil {
		t.Fatalf("gating: %v", err)
	}

	if len(kept) != 0 {
		t.Fatalf("kept %q, want every one dropped", titles(kept))
	}
}

// The failure 86cba69qh describes: one postmortem filing the same proposal
// twice in a single queue, at no model cost to catch.
func TestGateDropsATitleTheSameQueueAlreadyCarries(t *testing.T) {
	t.Parallel()

	queue := []clickup.Finding{
		{Title: "Poll before sleeping in the three waits", Body: "first"},
		{Title: "Poll before sleeping in the three waits", Body: "second"},
	}

	kept, err := clickup.RunGateForTest(queue, nil, never)
	if err != nil {
		t.Fatalf("gating: %v", err)
	}

	if len(kept) != 1 || kept[0].Body != "first" {
		t.Fatalf("kept %d row(s) %q, want only the first", len(kept), titles(kept))
	}
}

// A 60-rune prefix is enough to drop a filing on a ticket somebody may work,
// and not enough to drop one on a ticket that was retired or has shipped.
func TestGatePrefixPassLooksOnlyAtTicketsSomebodyMayWork(t *testing.T) {
	t.Parallel()

	title := "Tee the merge run's own stdout into the postmortem transcript"
	queue := []clickup.Finding{{Title: title}}

	for _, status := range []string{backlog, queued} {
		rows := []clickup.CorpusRow{{ID: older, Name: title, Status: status}}

		kept, err := clickup.RunGateForTest(queue, rows, never)
		if err != nil {
			t.Fatalf("%s: %v", status, err)
		}

		if len(kept) != 0 {
			t.Errorf("a %s ticket with the same title did not block the filing", status)
		}
	}

	for _, status := range []string{notRelevant, "done", "failed"} {
		rows := []clickup.CorpusRow{{ID: older, Name: title, Status: status}}

		kept, err := clickup.RunGateForTest(queue, rows, never)
		if err != nil {
			t.Fatalf("%s: %v", status, err)
		}

		if len(kept) != 1 {
			t.Errorf("a %s ticket blocked the filing on its title alone", status)
		}
	}
}

func TestGateStopsOnARankingItCannotGet(t *testing.T) {
	t.Parallel()

	failing := func(_ clickup.Finding, _ string) ([]clickup.Match, error) {
		return nil, errors.New("the turn timed out")
	}

	_, err := clickup.RunGateForTest([]clickup.Finding{{Title: first}}, nil, failing)
	if err == nil {
		t.Fatal("a queue judged by nothing was gated as though it had been judged")
	}
}

// scripts/merge/tickets.go:25 dies on a File error and leaves review threads
// unanswerable, which costs more than a duplicate the next sweep retires;
// `clickup defer` is a person who can be told to try again.
func TestABrokenGateFilesUngatedForFileAndNothingForTheStrictPaths(t *testing.T) {
	t.Parallel()

	queue := []clickup.Finding{{Title: first}, {Title: "two"}}
	broken := errors.New("the corpus could not be dumped")

	kept, err := clickup.DecideForTest(queue, broken, false)
	if err != nil {
		t.Fatalf("File refused to file on a broken gate: %v", err)
	}

	if len(kept) != len(queue) {
		t.Errorf("File filed %d of %d ungated, want all of them", len(kept), len(queue))
	}

	_, err = clickup.DecideForTest(queue, broken, true)
	if !errors.Is(err, clickup.ErrNotFiled) {
		t.Fatalf("a strict caller returned %v, want ErrNotFiled", err)
	}
}

func TestRankPromptNamesTheCorpusTheCandidateAndTheExclusion(t *testing.T) {
	t.Parallel()

	candidate := clickup.Finding{Title: "Bound the extract", Body: "by each section's stamp"}

	prompt := clickup.RankPromptForTest(candidate, "")
	for _, want := range []string{clickup.CorpusDir, candidate.Title, candidate.Body} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the similarity prompt does not carry %q:\n%s", want, prompt)
		}
	}

	if strings.Contains(prompt, "Never return it") {
		t.Error("a filing candidate is not in the corpus, so it needs no exclusion")
	}

	if audit := clickup.RankPromptForTest(candidate, "86self"); !strings.Contains(audit, "86self") {
		t.Errorf("an audited ticket is not excluded from its own matches:\n%s", audit)
	}
}

// A set of kept titles would hand both sections back and file both, because
// they share the title the intra-queue pass kept exactly one of.
func TestASectionTitleFiledTwiceInOneDocumentSurvivesOnce(t *testing.T) {
	t.Parallel()

	document := "## Poll before sleeping\n\nfirst\n\n## Poll before sleeping\n\nsecond\n"
	kept := []clickup.Finding{{Title: "Poll before sleeping"}}

	got := clickup.SurvivingSectionsForTest(document, kept)
	if len(got) != 1 {
		t.Fatalf("%d section(s) survived %v, want 1", len(got), got)
	}
}

func TestSurvivingKeepsTheSectionsTheGateDidNotDrop(t *testing.T) {
	t.Parallel()

	document := "## one\n\na\n\n## two\n\nb\n\n## three\n\nc\n"
	kept := []clickup.Finding{{Title: first}, {Title: third}}

	got := clickup.SurvivingSectionsForTest(document, kept)
	if len(got) != 2 || got[0] != first || got[1] != third {
		t.Fatalf("survived %v, want [one three] in document order", got)
	}
}
