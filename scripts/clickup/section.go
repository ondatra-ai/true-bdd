package clickup

import (
	"log/slog"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/triage"
)

// section is one `## ` heading of a hand-written deferral and everything under
// it: one ticket, before it has been judged.
type section struct {
	Title string
	// Body is the section WITHOUT its `## ` line — the four `### ` headings a
	// refresh rewrites and hands back in the same shape.
	Body  string
	Score int
}

// render is the section as the filing turn reads it again.
func (s section) render() string {
	return "## " + s.Title + "\n\n" + strings.TrimSpace(s.Body) + "\n"
}

// splitSections cuts a document into its tickets. It splits on exactly what
// countHeadings counts, fenced `## ` included, so the two can never disagree
// about how many tickets a document holds.
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

// scorer is triage.Score, taken as a parameter so the keep-or-drop decision
// below is reachable from a test — the turn itself is not.
type scorer func(triage.Subject) (triage.Verdict, error)

// triageSections keeps what reaches the floor, with the refreshed body. An
// unscored section is dropped, not filed: that is what this path did before,
// and it left every deferral unsortable by task-loop.
func triageSections(sections []section, score scorer) []section {
	kept := make([]section, 0, len(sections))

	for _, held := range sections {
		verdict, err := score(subjectOf(held))
		if err != nil {
			slog.Error("A ticket could not be scored and was not filed",
				"title", held.Title, "error", err)

			continue
		}

		if verdict.Score < triage.Floor {
			slog.Warn("Not filed: below the floor", "title", held.Title,
				"score", verdict.Score, "floor", triage.Floor, "reason", verdict.Reason)

			continue
		}

		held.Score = verdict.Score
		held.Body = verdict.Description

		kept = append(kept, held)
	}

	return kept
}

// subjectOf is where this caller's subject comes from. Refresh is set: the
// body already carries the four headings a refresh rewrites, and a deferral
// written from memory is exactly the thing worth checking against the code.
func subjectOf(held section) triage.Subject {
	return triage.Subject{
		ID:      held.Title,
		Title:   held.Title,
		Body:    held.Body,
		Origin:  "a hand-written deferral",
		Refresh: true,
	}
}
