package clickup

import (
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
)

// dropped is one candidate the gate refused, and the ticket it already is.
type dropped struct {
	Finding Finding
	Match   Match
}

// gated is the dedup filter every filing path runs BEFORE the render, so the
// heading count, the field plan and report's check agree on one queue.
func gated(queue []Finding, strict bool) ([]Finding, error) {
	kept, err := loaded(queue)

	return decide(queue, kept, err, strict)
}

// loaded dumps the corpus and runs the passes against it.
func loaded(queue []Finding) ([]Finding, error) {
	rows, err := dumpCorpus()
	if err != nil {
		return nil, err
	}

	return runGate(queue, rows, judge{byID: index(rows)}.rank)
}

// decide is what a gate that could not answer means: file nothing, or file
// ungated.
func decide(queue, kept []Finding, err error, strict bool) ([]Finding, error) {
	if err == nil {
		return kept, nil
	}

	if strict {
		return nil, fmt.Errorf("%w: %w", ErrNotFiled, err)
	}

	// scripts/merge/tickets.go:25 dies on a File error, and an aborted merge
	// leaves review threads unanswerable — worse than a duplicate the next
	// sweep retires.
	slog.Error("The duplicate gate could not answer; filing this queue ungated",
		"queued", len(queue), "error", err)

	return queue, nil
}

// runGate runs the three passes, cheapest first. The corpus and the ranker are
// parameters so this is reachable from a test — neither turn is.
func runGate(queue []Finding, rows []CorpusRow, rank ranker) ([]Finding, error) {
	kept, already := dropAlreadyOpen(queue, openRows(rows))
	for _, task := range already {
		slog.Info("Not filed: already open", "ticket", task.ID, "title", task.Name)
	}

	kept, repeats := withoutRepeats(kept)
	for _, finding := range repeats {
		slog.Warn("Not filed: this queue already carries it", "title", finding.Title)
	}

	kept, refused, err := judged(kept, rank)
	if err != nil {
		return nil, err
	}

	for _, drop := range refused {
		slog.Warn("Not filed: duplicate", "title", drop.Finding.Title,
			"of", drop.Match.ID, "url", drop.Match.URL, "status", drop.Match.Status,
			"score", drop.Match.Score, "reason", drop.Match.Reason)
	}

	return kept, nil
}

// openRows narrows the corpus to what a title prefix may suppress. A retired
// or closed ticket is left to the semantic pass: a 60-rune prefix is not
// evidence enough to drop a filing on a ticket nobody is going to work.
func openRows(rows []CorpusRow) []Task {
	open := make([]Task, 0, len(rows))

	for _, row := range rows {
		if row.Status != backlogStatus && row.Status != queuedStatus {
			continue
		}

		open = append(open, Task{
			ID: row.ID, Name: row.Name, Status: row.Status, URL: row.URL,
			Created: row.Created, TriageDate: row.TriageDate,
		})
	}

	return open
}

// withoutRepeats drops a candidate whose title prefix another candidate in the
// SAME queue already claimed, at no model cost.
func withoutRepeats(queue []Finding) ([]Finding, []Finding) {
	claimed := make(map[string]struct{}, len(queue))
	kept := make([]Finding, 0, len(queue))

	var repeats []Finding

	for _, finding := range queue {
		prefix := textutil.Truncate(finding.Title, matchWidth)

		_, taken := claimed[prefix]
		if taken {
			repeats = append(repeats, finding)

			continue
		}

		claimed[prefix] = struct{}{}

		kept = append(kept, finding)
	}

	return kept, repeats
}

// judged drops every candidate an existing ticket already covers. A ranking
// that fails stops the pass: a candidate judged by nothing is ungated, which
// is the caller's decision to make and not this loop's.
func judged(queue []Finding, rank ranker) ([]Finding, []dropped, error) {
	kept := make([]Finding, 0, len(queue))

	var refused []dropped

	for _, finding := range queue {
		matches, err := rank(finding, "")
		if err != nil {
			return nil, nil, err
		}

		best := top(matches)
		if best.Score > fileableCeiling {
			refused = append(refused, dropped{Finding: finding, Match: best})

			continue
		}

		kept = append(kept, finding)
	}

	return kept, refused, nil
}
