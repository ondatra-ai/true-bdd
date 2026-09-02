package merge

import (
	"encoding/json"
	"fmt"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
	"github.com/ondatra-ai/true-bdd/scripts/report"
	"github.com/ondatra-ai/true-bdd/scripts/triage"
)

// findingLimit caps how much of a finding the scorer is shown.
const findingLimit = 2500

// triage scores every finding against the tree as it stands. One turn each:
// the shared scorer reads the code before it answers, which is what lets it
// score a finding that misreads this codebase a 1 rather than guessing.
func (r *Run) triage(findings []clickup.Finding, round int) []clickup.Finding {
	defer report.Open("triage", "findings", len(findings))()

	if len(findings) == 0 {
		return nil
	}

	for index, finding := range findings {
		verdict, err := triage.Score(r.subjectOf(finding))
		if err != nil {
			r.dief("scoring %s: %v", finding.ID, err)
		}

		// Only what the verdict judged and wrote is carried back. The body
		// stays the reviewer's own words, which the fix agent and the thread
		// reply are both anchored to.
		findings[index].Score = verdict.Score
		findings[index].Reason = verdict.Reason
		findings[index].Story = verdict.Story
	}

	r.save(r.roundDir(round)+"/scored.json", findings)

	return findings
}

// subjectOf is the whole difference between merge's caller and the others:
// where the subject comes from. Filed stays unset — this creates a ticket, so
// the turn is asked for a story and the body stays the reviewer's own words.
func (r *Run) subjectOf(finding clickup.Finding) triage.Subject {
	// A postmortem proposal arrives with no id — the prompt returns none — and
	// the id is only ever a label in a diagnostic, so the title stands in.
	label := finding.ID
	if label == "" {
		label = finding.Title
	}

	return triage.Subject{
		ID:       label,
		Title:    finding.Title,
		Body:     truncate(finding.Body, findingLimit),
		File:     finding.File,
		Line:     finding.Line,
		Origin:   fmt.Sprintf("%s, on PR #%d", finding.Source, r.pr),
		Severity: finding.Severity,
	}
}

// split decides what happens to each finding, against r.floors — manual by
// default, automatic under a mandate. Round 3 fixes nothing either way.
// Body-only findings never fix inline: no thread or diff position to report a fix back to.
func (r *Run) split(
	issues []clickup.Finding, round int,
) ([]clickup.Finding, []clickup.Finding, []clickup.Finding) {
	var toFix, toCreate, toIgnore []clickup.Finding

	terminal := round > lastFixRound

	for _, finding := range issues {
		switch {
		case finding.Score < r.floors.Ticket:
			toIgnore = append(toIgnore, finding)
		case finding.Score >= r.floors.Fix && !terminal && finding.Source == "thread":
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

	var table strings.Builder

	table.WriteString("| Score | Band | Finding | File:line | Source |\n")
	table.WriteString("|-------|------|---------|-----------|--------|\n")

	for _, row := range rows {
		title := strings.ReplaceAll(truncate(row.finding.Title, bandTitleWidth), "|", `\|`)
		fmt.Fprintf(&table, "| %d | %s | %s | `%s:%s` | %s |\n",
			row.finding.Score, row.band, title, row.finding.File, row.finding.Line, row.finding.Source)
	}

	fmt.Fprintf(&table, "\n**%d to fix, %d ticketed, %d ignored**\n",
		len(toFix), len(toCreate), len(toIgnore))

	// A markdown table is an answer pr-merge reads, and per-line log framing
	// would destroy it. It goes to a file; the log names the path.
	path := filepath.Join(StateDir, "triage.md")

	err := disk.Write(path, []byte(table.String()), disk.Shared)
	if err != nil {
		r.logf("could not write the triage table: %v", err)

		return
	}

	slog.Info("Triage table written",
		"path", path, "fix", len(toFix), "ticket", len(toCreate), "ignore", len(toIgnore))
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
