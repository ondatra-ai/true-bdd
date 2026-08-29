package taskhandle

import (
	"log/slog"
	"strings"
)

// report is the outcome line and the checklist. Narrated one slog record per
// line: a record carrying newlines is escaped into a single unreadable line by
// slog's text handler, and this list exists to be read.
func (r *Run) report(outcome Outcome) {
	r.save("outcome.md", r.outcomeReport(outcome))

	for _, text := range strings.Split(r.outcomeReport(outcome), "\n") {
		slog.Info(text)
	}
}

// outcomeReport is the one line, then every step.
func (r *Run) outcomeReport(outcome Outcome) string {
	return r.outcomeLine(outcome) + "\n\n" + r.list.Render()
}

func (r *Run) outcomeLine(outcome Outcome) string {
	parts := []string{r.ticketID}

	if r.detail.Name != "" {
		parts = append(parts, r.detail.Name)
	}

	parts = append(parts, string(outcome))

	if r.pullRequest != "" {
		parts = append(parts, r.pullRequest)
	}

	return strings.Join(parts, " · ")
}
