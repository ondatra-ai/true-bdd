package merge

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
)

const (
	// findingLimit caps how much of a finding the scorer is shown.
	findingLimit        = 2500
	defaultScoreTimeout = 900 * time.Second
	// missingSample is how many unscored ids a stop names before it is a wall
	// of text rather than a diagnosis.
	missingSample = 5
	// The rubric's band, checked rather than trusted.
	minScore = 1
	maxScore = 10
)

// scoreTimeout bounds the one call that scores every finding.
func scoreTimeout() time.Duration { return envDuration("MERGE_SCORE_TIMEOUT", defaultScoreTimeout) }

// scored is one row of the scoring answer. Score is `any` because a model
// that answers "9", 9.0 or 9 must all be read the same way, and one that
// answers "high" must be a stop rather than a zero.
type scored struct {
	ID     string `json:"id"`
	Score  any    `json:"score"`
	Reason string `json:"reason"`
}

// triage scores every finding in one model call. Anything unscored is a stop.
func (r *Run) triage(findings []clickup.Finding, round int) []clickup.Finding {
	if len(findings) == 0 {
		return nil
	}

	verdicts := r.scoreAll(findings)

	var missing []string

	for _, finding := range findings {
		if _, ok := verdicts[finding.ID]; !ok {
			missing = append(missing, finding.ID)
		}
	}

	if len(missing) > 0 {
		r.dief("scoring returned no verdict for %d finding(s): %v",
			len(missing), missing[:min(missingSample, len(missing))])
	}

	for index := range findings {
		row := verdicts[findings[index].ID]

		score, ok := asScore(row.Score)
		if !ok {
			r.dief("scoring returned a non-numeric score %v for %s", row.Score, findings[index].ID)
		}

		if score < minScore || score > maxScore {
			r.dief("scoring returned %d for %s — outside %d-%d",
				score, findings[index].ID, minScore, maxScore)
		}

		findings[index].Score = score
		findings[index].Reason = row.Reason
	}

	r.save(r.roundDir(round)+"/scored.json", findings)

	return findings
}

// scoreAll is the one model call, keyed by finding id.
func (r *Run) scoreAll(findings []clickup.Finding) map[string]scored {
	type row struct {
		ID               string `json:"id"`
		File             string `json:"file"`
		Line             string `json:"line"`
		ReviewerSeverity string `json:"reviewer_severity"`
		Finding          string `json:"finding"`
	}

	payload := make([]row, 0, len(findings))
	for _, finding := range findings {
		payload = append(payload, row{
			ID: finding.ID, File: finding.File, Line: finding.Line,
			ReviewerSeverity: finding.Severity,
			Finding:          truncate(finding.Body, findingLimit),
		})
	}

	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		r.dief("encoding the findings for scoring: %v", err)
	}

	answer, err := claudecli.Run(rubricPrompt+"\n\nFindings:\n"+string(encoded), claudecli.Options{
		Role:    "merge-triage",
		Timeout: scoreTimeout(),
	})
	if err != nil {
		r.dief("scoring failed: %v", err)
	}

	verdicts := map[string]scored{}

	for _, item := range parseJSONArrayInto[scored](r, answer, "scoring") {
		if item.ID != "" {
			verdicts[item.ID] = item
		}
	}

	return verdicts
}

// asScore reads what a model called a score.
func asScore(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))

		return parsed, err == nil
	default:
		return 0, false
	}
}

// split decides what happens to each finding.
//
// Rounds 1-2: >=9 fixed, 6-8 ticketed, <6 dropped.
// Round 3   : nothing fixed, >=6 ticketed, <6 dropped. An empty fix band is
// what ends the loop, so this is also what guarantees the run never exceeds
// three rounds — Main stops on its own once a round plans no fixes.
//
// A body-only finding is never fixed inline whatever it scores: it has no
// thread to answer and no diff position, so the fix could not be reported back
// even if it were made.
func (r *Run) split(
	issues []clickup.Finding, round int,
) ([]clickup.Finding, []clickup.Finding, []clickup.Finding) {
	var toFix, toCreate, toIgnore []clickup.Finding

	terminal := round > lastFixRound

	for _, finding := range issues {
		switch {
		case finding.Score < ticketFloor:
			toIgnore = append(toIgnore, finding)
		case finding.Score >= fixFloor && !terminal && finding.Source == "thread":
			toFix = append(toFix, finding)
		default:
			toCreate = append(toCreate, finding)
		}
	}

	r.printTable(toFix, toCreate, toIgnore)

	return toFix, toCreate, toIgnore
}

func (r *Run) printTable(toFix, toCreate, toIgnore []clickup.Finding) {
	const bandTitleWidth = 80

	type banded struct {
		band    string
		finding clickup.Finding
	}

	rows := make([]banded, 0, len(toFix)+len(toCreate)+len(toIgnore))
	for _, group := range []struct {
		band     string
		findings []clickup.Finding
	}{
		{"Fix now", toFix}, {"Ticket", toCreate}, {"Ignore", toIgnore},
	} {
		for _, finding := range group.findings {
			rows = append(rows, banded{band: group.band, finding: finding})
		}
	}

	if len(rows) == 0 {
		r.logf("no findings")

		return
	}

	_, _ = fmt.Fprintln(os.Stdout)
	_, _ = fmt.Fprintln(os.Stdout, "| Score | Band | Finding | File:line | Source |")
	_, _ = fmt.Fprintln(os.Stdout, "|-------|------|---------|-----------|--------|")

	for _, row := range rows {
		title := strings.ReplaceAll(truncate(row.finding.Title, bandTitleWidth), "|", `\|`)
		_, _ = fmt.Fprintf(os.Stdout, "| %d | %s | %s | `%s:%s` | %s |\n",
			row.finding.Score, row.band, title, row.finding.File, row.finding.Line, row.finding.Source)
	}

	_, _ = fmt.Fprintf(os.Stdout, "\n**%d to fix, %d ticketed, %d ignored**\n\n",
		len(toFix), len(toCreate), len(toIgnore))
}

// answerLimit is how much of an unusable answer a stop quotes back.
const answerLimit = 500

// parseJSONArrayInto reads a model's array answer. Anything else is a stop.
func parseJSONArrayInto[T any](run *Run, answer, what string) []T {
	raw, err := textutil.ExtractJSONArray(answer)
	if err != nil {
		run.dief("%s produced no JSON array:\n%s", what, truncate(answer, answerLimit))
	}

	var items []T

	err = json.Unmarshal(raw, &items)
	if err != nil {
		run.dief("%s produced invalid JSON (%v):\n%s", what, err, truncate(string(raw), answerLimit))
	}

	return items
}
