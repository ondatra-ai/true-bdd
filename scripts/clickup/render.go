package clickup

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
)

// bodyLimit caps how much of a finding reaches the ticket.
const bodyLimit = 4000

// Render turns a queue into one markdown document, one `## ` heading per
// ticket — written so it can be picked up cold and the model's job is
// transcription, not authorship.
func Render(queue []Finding, tag, pr string) string {
	origin := "a local review"
	if pr != "" {
		origin = "PR #" + pr
	}

	// The header, then one fixed-height block per ticket.
	const (
		headerLines    = 6
		linesPerTicket = 31
	)

	lines := make([]string, 0, headerLines+len(queue)*linesPerTicket)
	lines = append(lines,
		"# ClickUp tickets to create",
		"",
		fmt.Sprintf("List: `%s`   Tag: `%s`   Source: %s", listID(), tag, origin),
		"",
		fmt.Sprintf("%d ticket(s). One ClickUp task per `## ` heading below.", len(queue)),
		"",
	)

	for index, finding := range queue {
		lines = append(lines, renderTicket(index+1, finding, origin)...)
	}

	return strings.Join(lines, "\n")
}

// renderTicket is the four headings the repository requires of a deferral:
// Why, What to change, Verification, Context.
func renderTicket(number int, finding Finding, origin string) []string {
	title := strings.TrimSpace(finding.Title)
	if title == "" {
		title = "Review finding"
	}

	reason := strings.TrimSpace(finding.Reason)
	if reason == "" {
		reason = "(no reason recorded)"
	}

	return []string{
		"---",
		"",
		"## " + strconv.Itoa(number) + ". " + title,
		"",
		"### Why",
		"",
		fmt.Sprintf("CodeRabbit raised this on %s; triage scored it **%d/10** — real, but not",
			origin, finding.Score),
		"worth blocking that merge on.",
		"",
		"> " + reason,
		"",
		"### What to change",
		"",
		fmt.Sprintf("`%s:%s`", orUnknown(finding.File), orUnknown(finding.Line)),
		"",
		textutil.Truncate(strings.TrimSpace(finding.Body), bodyLimit),
		"",
		"### Verification",
		"",
		"```bash",
		"go run ./scripts/cmd/gates run",
		"```",
		"",
		"### Context",
		"",
		"Deferred rather than fixed inline: the merge loop caps fixing at two",
		"review rounds, because CodeRabbit's free tier allows ~4 PR reviews an",
		"hour and PR #76 exhausted that by fixing everything inline over four",
		fmt.Sprintf("rounds. Reviewer severity `%s`, source `%s`.",
			orUnknown(finding.Severity), orUnknown(finding.Source)),
		"",
	}
}

// WriteRendered renders the queue to TicketsMarkdown and reports what it
// wrote.
func WriteRendered(queue []Finding, tag, pr string) (string, error) {
	document := Render(queue, tag, pr)

	err := os.MkdirAll(filepath.Dir(TicketsMarkdown), dirMode)
	if err != nil {
		return "", fmt.Errorf("creating %s: %w", filepath.Dir(TicketsMarkdown), err)
	}

	err = os.WriteFile(TicketsMarkdown, []byte(document), fileMode)
	if err != nil {
		return "", fmt.Errorf("writing %s: %w", TicketsMarkdown, err)
	}

	return document, nil
}
