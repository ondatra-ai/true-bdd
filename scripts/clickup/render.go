package clickup

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// Origin is where a queue came from, in the words the ticket shows. Prose, not
// a number, because a deferral has no pull request and "a local review" is not
// what it is: the caller knows its own provenance and nothing else does.
func Origin(pullRequest string) string {
	if pullRequest == "" {
		return "a local review"
	}

	return "PR #" + pullRequest
}

// Render turns a queue into one markdown document, one `## ` heading per
// ticket — written so it can be picked up cold and the model's job is
// transcription, not authorship. Every source renders through here.
func Render(queue []Finding, tag, origin string) string {
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

// renderTicket is one ticket: its `## ` title, then the headings ticket.yaml
// declares. The wording lives there; this decides only the title.
func renderTicket(number int, finding Finding, origin string) []string {
	title := strings.TrimSpace(finding.Title)
	if title == "" {
		title = "Review finding"
	}

	headings := renderHeadings(finding, origin)

	const titleLines = 4

	lines := make([]string, 0, titleLines+len(headings))
	lines = append(lines, "---", "", "## "+strconv.Itoa(number)+". "+title, "")

	return append(lines, headings...)
}

// WriteRendered renders the queue to TicketsMarkdown and reports what it
// wrote.
func WriteRendered(queue []Finding, tag, origin string) (string, error) {
	document := Render(queue, tag, origin)

	err := disk.Write(TicketsMarkdown, []byte(document), disk.Shared)
	if err != nil {
		return "", err
	}

	return document, nil
}

// countHeadings counts what the filing turn is told to create. A `## ` inside
// a rendered body counts too, which is why fileQueue holds it against the
// queue length: a body that embeds one asks the turn for an extra task.
func countHeadings(document string) int {
	count := 0

	for line := range strings.SplitSeq(document, "\n") {
		if strings.HasPrefix(line, "## ") {
			count++
		}
	}

	return count
}
