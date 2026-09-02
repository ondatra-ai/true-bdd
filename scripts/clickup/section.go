package clickup

import (
	"log/slog"
	"strings"
)

// deferralSource marks a candidate a person wrote by hand, which is the only
// source that arrives unscored and the only one with no file to bound it.
const deferralSource = "deferral"

// section is one `## ` heading of a hand-written deferral and everything under
// it: a title and raw prose, before it has been judged or shaped.
type section struct {
	Title string
	// Body is the section WITHOUT its `## ` line. It is raw material for the
	// ticket's `### What to change`, not a ticket: ticket.yaml decides shape.
	Body string
}

// splitSections cuts a document into its tickets, on `## ` and nothing else —
// a `## ` inside a fenced block opens one too, which countHeadings then sees
// in the render, so the two can never disagree about how many tickets there are.
func splitSections(document string) []section {
	var (
		sections []section
		current  *section
		body     strings.Builder
	)

	closeCurrent := func() {
		if current == nil {
			return
		}

		current.Body = body.String()
		sections = append(sections, *current)

		body.Reset()
	}

	for line := range strings.SplitSeq(document, "\n") {
		if strings.HasPrefix(line, "## ") {
			closeCurrent()

			current = &section{Title: strings.TrimSpace(strings.TrimPrefix(line, "## "))}

			continue
		}

		if current != nil {
			body.WriteString(line + "\n")
		}
	}

	closeCurrent()

	return sections
}

// findingsOf turns a parsed deferral into candidates for the one creator. The
// prose becomes Body and nothing else: a deferral that writes its own `### `
// headings is the old shape, and would nest them inside the rendered ones.
func findingsOf(sections []section) []Finding {
	queue := make([]Finding, 0, len(sections))

	for _, held := range sections {
		body := strings.TrimSpace(held.Body)
		if strings.Contains(body, "### ") {
			slog.Warn("This deferral writes its own `### ` headings; ticket.yaml renders them now",
				"title", held.Title)
		}

		queue = append(queue, Finding{Title: held.Title, Body: body, Source: deferralSource})
	}

	return queue
}
