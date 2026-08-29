package clickup

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
	"github.com/ondatra-ai/true-bdd/scripts/triage"
)

var (
	errNothingToTriage = errors.New("a sweep needs a count of at least one ticket")
	errNotTriaged      = errors.New("tickets were walked but not triaged")
	errApplyRefused    = errors.New("the ticket was not updated")
)

const applyPromptTemplate = `Using the ClickUp MCP tools, update task %s and nothing else.

1. Set its status to %q.
%s
3. Set these custom fields with setCustomFieldValue:
   - field %s — the integer %s. It addresses the Triage Score dropdown option
     by POSITION, so passing the score itself sets the wrong one.
   - field %s — the integer %d. It is a date field, which takes unix
     MILLISECONDS.
   - field %s — the string %q.%s

Change NOTHING else: not the name, not the tags, not the assignees, and no
custom field beyond the three above.

Answer with the result of those steps, and nothing you did not do.
`

// applySchema is what the turn is held to. A prose answer was parsed here
// once and "All four writes succeeded.\n\nOK" read as a refusal, because the
// OK was not the prefix — the writes had all landed.
const applySchema = `{"type":"object",` +
	`"required":["ok","error"],` +
	`"properties":{` +
	`"ok":{"type":"boolean"},` +
	`"error":{"type":"string"}}}`

// applied is the turn's answer.
type applied struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

// The description step, which is only taken when a refresh came back. Numbered
// so the turn reads three steps either way.
const (
	keepDescription = "2. Leave its description exactly as it is."
	setDescription  = "2. Replace its description with the markdown between the BEGIN and END\n" +
		"   markers below, verbatim. Do not summarise, extend or reformat it."
	descriptionBlock = "\n\n--- BEGIN DESCRIPTION ---\n%s\n--- END DESCRIPTION ---\n"
)

// apply writes one verdict back. The status is only ever moved DOWN to
// `not relevant` — a ticket a person promoted to `to do` stays there, because
// a sweep judges whether the work is still real, not whether it is queued.
func apply(ticket Task, verdict triage.Verdict, taken stamp) error {
	status := dispositionOf(verdict, ticket.Status)

	raw, err := claudecli.RunJSON(applyPrompt(ticket, verdict, taken, status), claudecli.Options{
		AllowedTools: triageTools,
		Schema:       applySchema,
		Role:         roleClickUp,
		Timeout:      claudeTimeout(),
	})
	if err != nil {
		return fmt.Errorf("updating %s: %w", ticket.ID, err)
	}

	var answer applied

	err = json.Unmarshal(raw, &answer)
	if err != nil {
		return fmt.Errorf("%w: unreadable answer (%w):\n%s",
			errApplyRefused, err, textutil.Truncate(string(raw), diagnosticLimit))
	}

	if !answer.OK {
		return fmt.Errorf("%w: %s", errApplyRefused, textutil.Truncate(answer.Error, diagnosticLimit))
	}

	slog.Info("Ticket triaged", "ticket", ticket.ID, "score", verdict.Score,
		"status", status, "refreshed", verdict.Description != "")

	return nil
}

// applyPrompt is the turn, built. Split out so a test can read it without a
// ClickUp round trip.
func applyPrompt(ticket Task, verdict triage.Verdict, taken stamp, status string) string {
	step, block := keepDescription, ""
	if strings.TrimSpace(verdict.Description) != "" {
		step = setDescription
		block = fmt.Sprintf(descriptionBlock, verdict.Description)
	}

	// A score outside the band would leave the dropdown alone, which validate
	// has already ruled out — the prompt still has to say what to pass.
	index := "left as it is"
	if position := orderindexOf(verdict.Score); position != nil {
		index = strconv.Itoa(*position)
	}

	return fmt.Sprintf(applyPromptTemplate, ticket.ID, status, step,
		triageScoreField, index, triageDateField, taken.Millis,
		triageCommitField, taken.Commit, block)
}
