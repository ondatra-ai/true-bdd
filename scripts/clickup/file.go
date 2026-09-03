package clickup

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
	"github.com/ondatra-ai/true-bdd/scripts/triage"
)

// ErrNotFiled reports that the queue and the ticket list disagree, in either
// direction. Both are failures of the one thing this queue exists to prevent.
var ErrNotFiled = errors.New("tickets were not filed")

// diagnosticLimit matches the quoting width the rest of the tooling uses.
const diagnosticLimit = 600

// matchWidth is the title prefix a queued finding and an open task are paired
// by — the same 60 runes ticketURL pairs a finding with its ticket at
// (scripts/merge/resolve.go:30).
const matchWidth = 60

// Ticket is one row of the filing turn's answer, and of FiledRecord.
type Ticket struct {
	Title  string `json:"title"`
	ID     string `json:"id"`
	URL    string `json:"url"`
	Status string `json:"status"`
	// FieldsSet reports whether the custom fields landed. A task whose
	// fields were refused still exists, so this is a warning, not a failure.
	FieldsSet bool   `json:"fields_set"`
	Error     string `json:"error"`
}

const filePromptTemplate = `Create ClickUp tasks from the markdown document below, using the ClickUp MCP
tools. Exactly one task per ` + "`## `" + ` heading — %d in total.

For each:
  - list id: %s
  - name: the heading text, without its leading number
  - description: everything under that heading, verbatim. That PLAIN-TEXT
    parameter, never markdownContent: ClickUp parses markdownContent into rich
    content, and getTask then returns the body flattened, with the ` + "`### `" + `
    markers every later reader needs stripped out of it.
  - status: %s
  - tag: %s

%s

Then set that task's custom fields with setCustomFieldValue, from the FIELDS
row whose "ticket" is that heading's number:
  - field %s — the "triage_score_orderindex" integer, passed VERBATIM. It
    addresses the dropdown option by POSITION, so passing the score instead
    sets the wrong one. When the row omits it, leave this field alone.
  - field %s — the "expected_changes" string. When the row omits it, leave
    this field alone: nothing bounded that change, and a person must.
  - field %s — the "triage_date_millis" integer, passed VERBATIM. It is a
    ClickUp date field, which takes unix MILLISECONDS.
  - field %s — the "triage_commit" string. When the row omits it, leave this
    field alone.

Set no other custom field: Scope and Good For Agent are a person's to fill.

Transcribe. Do not summarise, rewrite, merge, split, reorder or improve any
ticket, and do not create a task that is not in the document. If a task
cannot be created, leave its "id" null and put the error in "error" — do not
retry silently and do not omit the row. A task that was created but whose
custom fields were refused is "fields_set": false, with the id still filled.

Return ONLY a JSON array, no prose and no code fence:
[{"title": "<heading text>", "id": "<clickup task id or null>",
  "url": "<url or null>", "status": "<the task's status after creation>",
  "fields_set": <true|false>, "error": "<empty if created>"}]

--- BEGIN FIELDS ---
%s
--- END FIELDS ---

--- BEGIN DOCUMENT ---
%s
--- END DOCUMENT ---
`

// File renders the queue and asks a headless turn to create one ClickUp task
// per heading. Gated, but never blocked by the gate: scripts/merge/tickets.go:25
// dies on the error, so a gate that cannot answer files the queue ungated.
func File(queuePath, tag, pullRequest string) error {
	return file(queuePath, tag, pullRequest, false)
}

// FileDeduped is the same filing under a gate that files NOTHING when it
// cannot answer. Both are gated now: "a finding recurring across PRs is news"
// left File open, and re-runs of one PR filed three pairs on 2026-09-01.
func FileDeduped(queuePath, tag, pullRequest string) error {
	return file(queuePath, tag, pullRequest, true)
}

// dropAlreadyOpen returns the findings no open task already covers, naming
// each one it drops. Dropping before the render keeps the heading count, the
// field plan and report's count check on the same shortened queue.
func dropAlreadyOpen(queue []Finding, open []Task) ([]Finding, []Task) {
	byTitle := make(map[string]Task, len(open))

	for _, task := range open {
		byTitle[textutil.Truncate(task.Name, matchWidth)] = task
	}

	kept := make([]Finding, 0, len(queue))

	var dropped []Task

	for _, finding := range queue {
		task, found := byTitle[textutil.Truncate(finding.Title, matchWidth)]
		if !found {
			kept = append(kept, finding)

			continue
		}

		dropped = append(dropped, task)
	}

	return kept, dropped
}

func file(queuePath, tag, pullRequest string, strict bool) error {
	queue, err := LoadQueue(queuePath)
	if err != nil {
		return err
	}

	if len(queue) == 0 {
		slog.Info("Nothing to file", "queue", queuePath)

		return nil
	}

	return fileQueue(queue, tag, Origin(pullRequest), strict)
}

// fileQueue is the ONE creator. A review finding, a postmortem proposal and a
// hand-written deferral all reach ClickUp through here, differing only in the
// queue they hand over and the origin they name.
func fileQueue(queue []Finding, tag, origin string, strict bool) error {
	kept, err := prepared(queue, strict)
	if err != nil || len(kept) == 0 {
		return err
	}

	document, err := WriteRendered(kept, tag, origin)
	if err != nil {
		return err
	}

	slog.Info("Tickets rendered", "count", len(kept), "path", TicketsMarkdown)

	// A body that embeds a `## ` asks the turn for a task the queue does not
	// have. report catches the fallout after the turn; this names it before.
	if headings := countHeadings(document); headings != len(kept) {
		slog.Warn("A rendered body embeds a `## `, so the turn is told a count the queue does not hold",
			"headings", headings, "queued", len(kept))
	}

	plans, err := encodeFields(planFields(kept, now()))
	if err != nil {
		return err
	}

	prompt := fmt.Sprintf(filePromptTemplate, len(kept), listID(), ticketStatus(), tag,
		statusRule(), triageScoreField, expectedChangesField, triageDateField,
		triageCommitField, plans, document)

	created, err := createTickets(prompt)
	if err != nil {
		return err
	}

	return report(len(kept), created)
}

// prepared is the two drops every source runs, in this order: the duplicate
// gate first, so a duplicate never spends a scoring turn, then the floor. An
// empty result is not an error — nothing was owed a ticket.
func prepared(queue []Finding, strict bool) ([]Finding, error) {
	kept, err := gated(queue, strict)
	if err != nil {
		return nil, err
	}

	if len(kept) == 0 {
		slog.Info("Nothing to file: the tracker already carries every ticket in this queue")

		return nil, nil
	}

	kept = scored(kept, triage.Score)
	if len(kept) == 0 {
		slog.Warn("Nothing to file: nothing reached the floor", "floor", triage.Floor)
	}

	return kept, nil
}

// encodeFields is the FIELDS block the filing prompt embeds.
func encodeFields(plans []fieldPlan) ([]byte, error) {
	encoded, err := json.MarshalIndent(plans, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding the custom fields: %w", err)
	}

	return encoded, nil
}

// createTickets runs the filing turn and reads the array back.
func createTickets(prompt string) ([]Ticket, error) {
	answer, err := claudecli.Run(prompt, claudecli.Options{
		AllowedTools: createTools,
		Role:         roleClickUp,
		Timeout:      claudeTimeout(),
	})
	if err != nil {
		if errors.Is(err, claudecli.ErrTimeout) {
			return nil, fmt.Errorf("%w: the ClickUp turn %w — nothing was filed", ErrNotFiled, err)
		}

		return nil, fmt.Errorf("%w: %w", ErrNotFiled, err)
	}

	raw, err := textutil.ExtractJSONArray(answer)
	if err != nil {
		return nil, fmt.Errorf("ticket creation returned %w:\n%s",
			err, textutil.Truncate(answer, diagnosticLimit))
	}

	var created []Ticket

	err = json.Unmarshal(raw, &created)
	if err != nil {
		return nil, fmt.Errorf("ticket creation returned invalid JSON (%w):\n%s",
			err, textutil.Truncate(answer, diagnosticLimit))
	}

	return created, nil
}

// report writes the per-ticket record and decides whether the filing stands.
func report(wanted int, created []Ticket) error {
	filed, failed := split(created)

	err := saveRecord(created)
	if err != nil {
		return err
	}

	for _, ticket := range filed {
		slog.Info("Ticket filed", "ticket", ticket.ID, "title", ticket.Title)
	}

	for _, ticket := range failed {
		title := ticket.Title
		if title == "" {
			title = "?"
		}

		reason := ticket.Error
		if reason == "" {
			reason = "no id returned"
		}

		slog.Error("Ticket was not created", "title", title, "reason", reason)
	}

	// A count that does not match is a silent drop, which is the whole
	// failure this queue exists to prevent — so it is an error, not a note.
	if len(created) != wanted {
		slog.Error("Queue and result disagree; nothing here is trustworthy",
			"queued", wanted, "returned", len(created))

		return ErrNotFiled
	}

	if len(failed) > 0 {
		slog.Error("Tickets were not created", "failed", len(failed), "queued", wanted)

		return ErrNotFiled
	}

	unfilled := unfilled(filed)
	if len(unfilled) > 0 {
		slog.Warn("Custom fields were not set; fill them by hand",
			"tickets", strings.Join(unfilled, " "))
	}

	wrong := misplaced(filed)
	if len(wrong) > 0 {
		slog.Warn("Tickets are not in the backlog; move them back before a task-loop picks them up",
			"tickets", strings.Join(wrong, ", "))
	}

	slog.Info("Filing complete", "filed", len(filed), "record", FiledRecord)

	return nil
}

// misplaced names tickets the filing turn left outside the backlog. Not an
// error — the task exists — but naming it is the only thing between a misfiled
// proposal and an unattended run picking it up.
func misplaced(filed []Ticket) []string {
	var wrong []string

	for _, ticket := range filed {
		if !strings.EqualFold(ticket.Status, ticketStatus()) {
			wrong = append(wrong, ticket.ID+" ("+orUnknown(ticket.Status)+")")
		}
	}

	return wrong
}

// unfilled names the tickets whose custom fields were refused. Not an error:
// the task exists and a Ticket short a field halts task-handle rather than
// misleading it, so this asks for a hand-fill instead of failing a merge.
func unfilled(filed []Ticket) []string {
	var bare []string

	for _, ticket := range filed {
		if !ticket.FieldsSet {
			bare = append(bare, ticket.ID)
		}
	}

	return bare
}

func split(created []Ticket) ([]Ticket, []Ticket) {
	var filed, failed []Ticket

	for _, ticket := range created {
		if ticket.ID != "" {
			filed = append(filed, ticket)

			continue
		}

		failed = append(failed, ticket)
	}

	return filed, failed
}

func saveRecord(created []Ticket) error {
	payload, err := json.MarshalIndent(created, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding the filing record: %w", err)
	}

	err = disk.Write(FiledRecord, payload, disk.Shared)
	if err != nil {
		return err
	}

	return nil
}
