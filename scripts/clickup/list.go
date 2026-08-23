package clickup

import (
	"encoding/json"
	"fmt"
	"io"

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
func List(out io.Writer, tag string) error {
	answer, err := claudecli.Run(fmt.Sprintf(listPromptTemplate, listID(), tag), claudecli.Options{
		AllowedTools: listTools,
		Role:         "clickup",
		Timeout:      claudeTimeout(),
	})
	if err != nil {
		return fmt.Errorf("listing the queue: %w", err)
	}

	raw, err := textutil.ExtractJSONArray(answer)
	if err != nil {
		return fmt.Errorf("ticket listing returned %w:\n%s",
			err, textutil.Truncate(answer, diagnosticLimit))
	}

	var tasks []Task

	err = json.Unmarshal(raw, &tasks)
	if err != nil {
		return fmt.Errorf("ticket listing returned invalid JSON (%w):\n%s",
			err, textutil.Truncate(answer, diagnosticLimit))
	}

	for _, task := range tasks {
		_, _ = fmt.Fprintf(out, "%s\t%s\t%s\n", task.ID, task.Status, task.Name)
	}

	if len(tasks) == 0 {
		_, _ = fmt.Fprintln(out, "(queue empty)")
	}

	return nil
}
