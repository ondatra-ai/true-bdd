package clickup

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
)

// The Task Execution log: every task-handle run, whatever its outcome, is one
// appended entry. A run nobody logged is a run nobody can audit.
const (
	logWorkspaceID = "90151491867"
	logDocID       = "2kyq568v-34735"
	logPageID      = "2kyq568v-25235"
)

// appendTools is the whole allowance, so a turn that decided the doc needed
// tidying could not act on it. append-only by construction, not by prompt.
const appendTools = "mcp__claude_ai_ClickUP__editPage"

const appendSchema = `{"type":"object","required":["appended"],` +
	`"properties":{"appended":{"type":"boolean"},"error":{"type":"string"}}}`

const appendPromptTemplate = `Use the ClickUp MCP editPage tool to APPEND this entry to the Task
Execution log page.

  workspaceId  %s
  docId        %s
  pageId       %s
  editMode     append

editMode MUST be "append". "replace" would drop every earlier entry, which is
the whole history of every run this workflow has ever made.

Append this content, verbatim, and nothing else — do not summarise it, do not
reformat it, do not add a preamble and do not tidy the page:

%s

Report {"appended": true} once the tool has confirmed the edit, or
{"appended": false, "error": "<what the tool said>"} if it refused.
`

var errAppendRefused = errors.New("the Task Execution log was not appended")

// AppendLog adds one entry to the Task Execution log. The caller decides what a
// failure means: task-handle reports it and prints the entry, because by then
// the run is over and there is nothing left to halt.
func AppendLog(entry string) error {
	prompt := fmt.Sprintf(appendPromptTemplate,
		logWorkspaceID, logDocID, logPageID, entry)

	raw, err := claudecli.RunJSON(prompt, claudecli.Options{
		AllowedTools: appendTools,
		Schema:       appendSchema,
		Role:         roleClickUp,
		Timeout:      claudeTimeout(),
	})
	if err != nil {
		return fmt.Errorf("appending to the Task Execution log: %w", err)
	}

	var answer struct {
		Appended bool   `json:"appended"`
		Error    string `json:"error"`
	}

	err = json.Unmarshal(raw, &answer)
	if err != nil {
		return fmt.Errorf("reading the append answer: %w", err)
	}

	if !answer.Appended {
		return fmt.Errorf("%w: %s", errAppendRefused, answer.Error)
	}

	return nil
}
