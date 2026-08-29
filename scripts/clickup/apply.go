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
   - field %s — the string %q.
4. ONLY if steps 1-3 all succeeded, add the text between the BEGIN COMMENT and
   END COMMENT markers with addTaskComment, verbatim, passing
   notifyAll: false. Do not summarise, extend or reformat it. If an earlier
   step failed, add no comment: a note claiming writes that did not land is
   worse than no note.

Change NOTHING else: not the name, not the tags, not the assignees, and no
custom field beyond the three above.

Answer with the result of those steps, and nothing you did not do. "ok" covers
steps 1-3; "commented" is step 4 alone, so a refused comment is not a failed
update.%s

--- BEGIN COMMENT ---
%s
--- END COMMENT ---
`

// applySchema is what the turn is held to. A prose answer was parsed here
// once and "All four writes succeeded.\n\nOK" read as a refusal, because the
// OK was not the prefix — the writes had all landed.
const applySchema = `{"type":"object",` +
	`"required":["ok","commented","error"],` +
	`"properties":{` +
	`"ok":{"type":"boolean"},` +
	`"commented":{"type":"boolean"},` +
	`"error":{"type":"string"}}}`

// applied is the turn's answer. Commented is separate so a refused comment
// can be reported without claiming the writes failed — the same split as
// Ticket.FieldsSet, and for the same reason.
type applied struct {
	OK        bool   `json:"ok"`
	Commented bool   `json:"commented"`
	Error     string `json:"error"`
}

// The description step, which is only taken when a refresh came back. Numbered
// so the turn reads four steps either way.
const (
	keepDescription = "2. Leave its description exactly as it is."
	setDescription  = "2. Replace its description with the markdown between the BEGIN DESCRIPTION\n" +
		"   and END DESCRIPTION markers below, verbatim. Do not summarise, extend\n" +
		"   or reformat it."
	descriptionBlock = "\n\n--- BEGIN DESCRIPTION ---\n%s\n--- END DESCRIPTION ---"
)

// refreshed reports whether a verdict came back with a body to write. The
// prompt branch and the note's Body line must not be able to disagree.
func refreshed(verdict triage.Verdict) bool {
	return strings.TrimSpace(verdict.Description) != ""
}

// apply writes one verdict back. The status is only ever moved DOWN to
// `not relevant` — a ticket a person promoted to `to do` stays there, because
// a sweep judges whether the work is still real, not whether it is queued.
func apply(ticket Task, was prior, verdict triage.Verdict, taken stamp) error {
	status := dispositionOf(verdict, ticket.Status)
	note := noteOf(was, verdict, taken, ticket.Status, status)

	raw, err := claudecli.RunJSON(applyPrompt(ticket, verdict, taken, status, note), claudecli.Options{
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

	// Not an error: the writes landed, and score.go already logged the reason
	// to the Task log. ClickUp is the convenience copy.
	if !answer.Commented {
		slog.Warn("The triage note was not added; the ticket was still updated",
			"ticket", ticket.ID, "error", textutil.Truncate(answer.Error, diagnosticLimit))
	}

	slog.Info("Ticket triaged", "ticket", ticket.ID, "score", verdict.Score,
		"status", status, "refreshed", refreshed(verdict))

	return nil
}

// applyPrompt is the turn, built. Split out so a test can read it without a
// ClickUp round trip.
func applyPrompt(ticket Task, verdict triage.Verdict, taken stamp, status, note string) string {
	step, block := keepDescription, ""
	if refreshed(verdict) {
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
		triageCommitField, taken.Commit, block, note)
}
