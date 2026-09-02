package clickup

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
)

// Task is one row of a listing turn's answer. Created and TriageDate are the
// two the sweep orders on, and are left empty by the queue listing, which asks
// for neither.
type Task struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	URL        string `json:"url"`
	Created    string `json:"created"`
	TriageDate string `json:"triage_date"`
}

const listPromptTemplate = `Use the ClickUp MCP tools to list every OPEN (not closed, not complete) task
in list %s carrying the tag ` + "`%s`" + `.

A task whose status is %s is NOT open; leave it out. Named rather than left to
judgement: listTasks returns a "not relevant" task even with includeClosed
false, so nothing but this sentence excludes a retired ticket.

Return ONLY a JSON array sorted OLDEST FIRST by creation date, no prose and
no code fence:
[{"id": "<id>", "name": "<name>", "status": "<status>", "url": "<url>"}]

Oldest first matters: this is a work queue, and ClickUp's default ordering
is newest-first, which leaves the oldest item permanently last.
If there are none, return [].
`

// settledStatuses are the statuses that end a ticket's life. Spelled into the
// prompt because listTasks returns a `not relevant` task even with
// includeClosed false (probed 2026-09-02), so nothing else excludes one.
const settledStatuses = `"` + notRelevantStatus + `", "` + doneStatus +
	`" or "` + failedStatus + `"`

// listPrompt names the list, the tag, and what does not count as open.
func listPrompt(tag string) string {
	return fmt.Sprintf(listPromptTemplate, listID(), tag, settledStatuses)
}

// List prints the open tasks carrying tag, oldest first.
func List(tag string) error {
	tasks, err := openTasks(tag)
	if err != nil {
		return err
	}

	for _, task := range tasks {
		slog.Info("Open ticket", "ticket", task.ID, "status", task.Status, "title", task.Name)
	}

	if len(tasks) == 0 {
		slog.Info("Queue empty", "tag", tag)
	}

	return nil
}

// openTasks runs the listing turn and reads its array back.
func openTasks(tag string) ([]Task, error) {
	answer, err := claudecli.Run(listPrompt(tag), claudecli.Options{
		AllowedTools: listTools,
		Role:         roleClickUp,
		Timeout:      claudeTimeout(),
	})
	if err != nil {
		return nil, fmt.Errorf("listing the queue: %w", err)
	}

	raw, err := textutil.ExtractJSONArray(answer)
	if err != nil {
		return nil, fmt.Errorf("ticket listing returned %w:\n%s",
			err, textutil.Truncate(answer, diagnosticLimit))
	}

	var tasks []Task

	err = json.Unmarshal(raw, &tasks)
	if err != nil {
		return nil, fmt.Errorf("ticket listing returned invalid JSON (%w):\n%s",
			err, textutil.Truncate(answer, diagnosticLimit))
	}

	return tasks, nil
}
