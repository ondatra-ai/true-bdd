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

Also return its "Triage Score": the NAME of the selected option — the number
ClickUp shows, 1 to 10 — and not the option's orderindex or its id. The 7th
option is named "7" and sits at orderindex 6, so reporting the position
reports the wrong score. Leave it "" when the field is unset.

Return ONLY a JSON array, no prose and no code fence:
[{"id": "<id>", "description": "<the full markdown description>",
  "score": "<the Triage Score option name, or empty>"}]

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

// prior is what a ticket said before this sweep judged it. Score stays raw as
// transcribed: it is only ever a line in a note, so a bad answer must not be
// able to skip a ticket the way an unreadable Description does.
type prior struct {
	Description string
	Score       string
}

// fetchBodies reads the full description of each selected ticket, keyed by id.
func fetchBodies(stale []Task) (map[string]prior, error) {
	ids := make([]string, 0, len(stale))
	for _, ticket := range stale {
		ids = append(ids, labelled(ticket.ID, ticket.Name))
	}

	return bodiesOf(ids)
}

// labelled is one line of the bodies turn's id block. The name rides along so
// a transcription error is visible in the prompt rather than only in the answer.
func labelled(id, name string) string {
	return "  - " + id + "  (" + name + ")"
}

// bodiesOf runs one bodies turn over an already-labelled batch of ids.
func bodiesOf(ids []string) (map[string]prior, error) {
	type body struct {
		ID          string `json:"id"`
		Description string `json:"description"`
		Score       string `json:"score"`
	}

	rows, err := listing[body](
		fmt.Sprintf(bodiesPromptTemplate, len(ids), strings.Join(ids, "\n")),
		listTools, "the ticket bodies")
	if err != nil {
		return nil, err
	}

	bodies := make(map[string]prior, len(rows))
	for _, row := range rows {
		bodies[row.ID] = prior{Description: row.Description, Score: row.Score}
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
