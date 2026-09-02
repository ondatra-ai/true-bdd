package clickup

import (
	"log/slog"

	"github.com/ondatra-ai/true-bdd/scripts/triage"
)

// scorer is triage.Score, taken as a parameter so the keep-or-drop decision
// below is reachable from a test — the turn itself is not.
type scorer func(triage.Subject) (triage.Verdict, error)

// scored judges what arrives unscored and drops what does not reach the floor.
// A row merge already scored is left alone: its disposition was decided against
// merge's own Floors table, which is not this one.
func scored(queue []Finding, score scorer) []Finding {
	kept := make([]Finding, 0, len(queue))

	for _, finding := range queue {
		if finding.Score > 0 {
			kept = append(kept, finding)

			continue
		}

		judged, keep := judgeOne(finding, score)
		if !keep {
			continue
		}

		kept = append(kept, judged)
	}

	return kept
}

// judgeOne scores one candidate. An unscorable one is dropped rather than
// filed unscored: that is what the deferral path did before, and it left every
// hand-written ticket unsortable by the queue task-loop works.
func judgeOne(finding Finding, score scorer) (Finding, bool) {
	verdict, err := score(subjectOf(finding))
	if err != nil {
		slog.Error("A ticket could not be scored and was not filed",
			"title", finding.Title, "error", err)

		return Finding{}, false
	}

	if verdict.Score < triage.Floor {
		slog.Warn("Not filed: below the floor", "title", finding.Title,
			"score", verdict.Score, "floor", triage.Floor, "reason", verdict.Reason)

		return Finding{}, false
	}

	finding.Score = verdict.Score
	finding.Reason = verdict.Reason
	finding.Story = verdict.Story

	return finding, true
}

// subjectOf is where an unscored candidate's subject comes from. Filed stays
// unset — this ticket does not exist yet, so the turn is asked for a story
// rather than for a rewrite of a body the tracker already holds.
func subjectOf(finding Finding) triage.Subject {
	return triage.Subject{
		ID:       finding.Title,
		Title:    finding.Title,
		Body:     finding.Body,
		File:     finding.File,
		Line:     finding.Line,
		Origin:   orUnknown(finding.Source),
		Severity: finding.Severity,
	}
}
