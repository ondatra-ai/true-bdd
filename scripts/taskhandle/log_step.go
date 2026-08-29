package taskhandle

import (
	"log/slog"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
)

// logRun appends this run to the Task Execution log, on EVERY outcome — the
// failures are the entries worth having. A failure HERE is only reported: the
// run is already over, so there is nothing left to halt and it is no FAILED.
func (r *Run) logRun(outcome Outcome) {
	entry := r.logEntry(outcome)

	err := clickup.AppendLog(entry)
	if err == nil {
		return
	}

	slog.Error("the Task Execution log was not appended — paste this entry by hand",
		"error", err)

	for _, text := range strings.Split(entry, "\n") {
		slog.Error(text)
	}
}

// logEntry is the heading, the outcome line and the checklist.
func (r *Run) logEntry(outcome Outcome) string {
	stamp := time.Now().UTC().Format("2006-01-02 15:04 UTC")

	heading := "## " + stamp + " · " + r.ticketID + " · " + string(outcome)

	subject := r.detail.Name
	if r.pullRequest != "" {
		subject += " — PR " + r.pullRequest
	}

	if r.sha != "" {
		subject += " `" + r.sha + "`"
	}

	return strings.Join([]string{heading, "", subject, "", r.list.Render()}, "\n")
}
