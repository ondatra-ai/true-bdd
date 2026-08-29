package clickup

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
)

const walkPromptTemplate = `Use the ClickUp MCP tools to list the tasks in list %s whose status is
%s.

Page through them: listTasks returns at most 100 per page, so call it with
page 0, then 1, and so on until a page comes back empty. A list this size has
more than one page, and stopping at the first silently loses the rest.

For each task report its "Triage Date" custom field EXACTLY as the tool gives
it — a raw millisecond number, copied digit for digit — or "" when the field
is unset. Do not convert it to a date, and report no other custom field.

Return ONLY a JSON array, no prose and no code fence:
[{"id": "<id>", "name": "<name>", "status": "<status>", "url": "<url>",
  "created": "<ISO-8601 creation date>", "triage_date": "<raw ms, or empty>"}]

Transcribe. Do not sort, filter, deduplicate or omit a row — the caller orders
them and decides which ones it wants. If there are none, return [].
`

const bodiesPromptTemplate = `Use the ClickUp MCP getTask tool to read these %d tasks, by id:

%s

Return each one's FULL description, verbatim — listTasks truncates it and the
truncated form is unusable here.

Return ONLY a JSON array, no prose and no code fence:
[{"id": "<id>", "description": "<the full markdown description>"}]

Transcribe. Do not summarise, shorten or reformat a description, and do not
omit a row: leave "description" empty if a task cannot be read.
`

// walkPrompt names the list and the two statuses a sweep may re-judge.
func walkPrompt() string {
	statuses := `"` + backlogStatus + `" or "` + queuedStatus + `"`

	return fmt.Sprintf(walkPromptTemplate, listID(), statuses)
}

// walkableTasks lists every ticket a sweep may re-judge.
func walkableTasks() ([]Task, error) {
	tasks, err := listing[Task](walkPrompt(), listTools, "the ticket walk")
	if err != nil {
		return nil, err
	}

	return tasks, nil
}

// fetchBodies reads the full description of each selected ticket, keyed by id.
func fetchBodies(stale []Task) (map[string]string, error) {
	ids := make([]string, 0, len(stale))
	for _, ticket := range stale {
		ids = append(ids, "  - "+ticket.ID+"  ("+ticket.Name+")")
	}

	type body struct {
		ID          string `json:"id"`
		Description string `json:"description"`
	}

	rows, err := listing[body](
		fmt.Sprintf(bodiesPromptTemplate, len(stale), strings.Join(ids, "\n")),
		listTools, "the ticket bodies")
	if err != nil {
		return nil, err
	}

	bodies := make(map[string]string, len(rows))
	for _, row := range rows {
		bodies[row.ID] = row.Description
	}

	return bodies, nil
}

// listing runs one read-only turn and reads its array back. Both walks answer
// in the same shape, and neither is allowed to create or change anything.
func listing[T any](prompt, tools, what string) ([]T, error) {
	answer, err := claudecli.Run(prompt, claudecli.Options{
		AllowedTools: tools,
		Role:         roleClickUp,
		Timeout:      claudeTimeout(),
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}

	raw, err := textutil.ExtractJSONArray(answer)
	if err != nil {
		return nil, fmt.Errorf("%s returned %w:\n%s",
			what, err, textutil.Truncate(answer, diagnosticLimit))
	}

	var rows []T

	err = json.Unmarshal(raw, &rows)
	if err != nil {
		return nil, fmt.Errorf("%s returned invalid JSON (%w):\n%s",
			what, err, textutil.Truncate(answer, diagnosticLimit))
	}

	return rows, nil
}
