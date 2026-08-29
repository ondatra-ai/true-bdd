package clickup

import (
	"errors"
	"fmt"
)

const fetchPromptTemplate = `Use the ClickUp MCP getTask tool to read task %s.

Return its FULL description verbatim — listTasks truncates it and the
truncated form is unusable here — its status, its url, and these five custom
fields:

  - "Triage Score": the NAME of the selected option, the number ClickUp shows,
    1 to 10, and not the option's orderindex or its id. The 7th option is
    named "7" and sits at orderindex 6, so reporting the position reports the
    wrong score. "" when unset.
  - "Triage Date": EXACTLY as the tool gives it, a raw millisecond number
    copied digit for digit. Do not convert it to a date. "" when unset.
  - "Triage Commit": the sha as stored. "" when unset.
  - "Expected Changes": the field's text verbatim, newlines included. ""
    when unset.
  - "Good For Agent": true only when the checkbox is checked.

Return ONLY a JSON array holding exactly ONE object, no prose and no code
fence:
[{"id": "<id>", "name": "<name>", "status": "<status>", "url": "<url>",
  "description": "<the full markdown description>",
  "triage_score": "<option name, or empty>",
  "triage_date": "<raw ms, or empty>",
  "triage_commit": "<sha, or empty>",
  "expected_changes": "<verbatim, or empty>",
  "good_for_agent": <true or false>}]

Transcribe. Do not summarise, shorten or reformat anything, and never invent a
value for a field that is unset — "" and false are the answers for those.
Return [] if the task does not exist.
`

var errNoSuchTicket = errors.New("no such Ticket")

// Detail is one Ticket read whole: the status, the body, and the five fields
// ticket-schema.yaml requires before it may be taken unattended. Ticket is a
// ticket this package CREATED; this is one it read.
type Detail struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	URL             string `json:"url"`
	Description     string `json:"description"`
	TriageScore     string `json:"triage_score"`
	TriageDate      string `json:"triage_date"`
	TriageCommit    string `json:"triage_commit"`
	ExpectedChanges string `json:"expected_changes"`
	GoodForAgent    bool   `json:"good_for_agent"`
}

// Fetch reads one Ticket whole. The turn answers with an array because that is
// the shape listing reads; one row is the whole contract, and anything else
// means the turn answered about something other than what was asked.
func Fetch(ticketID string) (Detail, error) {
	rows, err := listing[Detail](
		fmt.Sprintf(fetchPromptTemplate, ticketID), listTools, "the ticket read")
	if err != nil {
		return Detail{}, err
	}

	if len(rows) != 1 {
		return Detail{}, fmt.Errorf("%w: reading %s returned %d rows, want 1",
			errNoSuchTicket, ticketID, len(rows))
	}

	return rows[0], nil
}
