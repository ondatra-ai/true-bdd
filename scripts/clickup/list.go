package clickup

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
)

// Task is one row of the listing turn's answer.
type Task struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	URL    string `json:"url"`
}

const listPromptTemplate = `Use the ClickUp MCP tools to list every OPEN (not closed, not complete) task
in list %s carrying the tag ` + "`%s`" + `.

Return ONLY a JSON array sorted OLDEST FIRST by creation date, no prose and
no code fence:
[{"id": "<id>", "name": "<name>", "status": "<status>", "url": "<url>"}]

Oldest first matters: this is a work queue, and ClickUp's default ordering
is newest-first, which leaves the oldest item permanently last.
If there are none, return [].
`

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
	answer, err := claudecli.Run(fmt.Sprintf(listPromptTemplate, listID(), tag), claudecli.Options{
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
