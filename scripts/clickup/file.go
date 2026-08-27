package clickup

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
)

// ErrNotFiled reports that the queue and the ticket list disagree, in either
// direction. Both are failures of the one thing this queue exists to prevent.
var ErrNotFiled = errors.New("tickets were not filed")

// diagnosticLimit matches the quoting width the rest of the tooling uses.
const diagnosticLimit = 600

// titleWidth caps a title in the filed/FAILED lines, wide and narrow.
const (
	filedTitleWidth  = 70
	failedTitleWidth = 60
)

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
  - markdownContent: everything under that heading, verbatim
  - status: %s
  - tag: %s

%s

Then set that task's custom fields with setCustomFieldValue, from the FIELDS
row whose "ticket" is that heading's number:
  - field %s — the "triage_score_orderindex" integer, passed VERBATIM. It
    addresses the dropdown option by POSITION, so passing the score instead
    sets the wrong one. When the row omits it, leave this field alone.
  - field %s — the "expected_changes" string.

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
// per heading.
func File(out, errOut io.Writer, queuePath, tag, pullRequest string) error {
	return file(out, errOut, queuePath, tag, pullRequest, false)
}

// FileDeduped files only what no open task under tag already covers. The
// postmortem's door: a review finding recurring across PRs is news, a process
// proposal recurring while its ticket sits unimplemented is a duplicate.
func FileDeduped(out, errOut io.Writer, queuePath, tag, pullRequest string) error {
	return file(out, errOut, queuePath, tag, pullRequest, true)
}

// dropAlreadyOpen returns the findings no open task already covers, naming
// each one it drops. Dropping before the render keeps the heading count, the
// field plan and report's count check on the same shortened queue.
func dropAlreadyOpen(out io.Writer, queue []Finding, open []Task) []Finding {
	byTitle := make(map[string]Task, len(open))

	for _, task := range open {
		byTitle[textutil.Truncate(task.Name, matchWidth)] = task
	}

	kept := make([]Finding, 0, len(queue))

	for _, finding := range queue {
		task, found := byTitle[textutil.Truncate(finding.Title, matchWidth)]
		if !found {
			kept = append(kept, finding)

			continue
		}

		_, _ = fmt.Fprintf(out, "already open %-12s %s\n",
			task.ID, textutil.Truncate(task.Name, filedTitleWidth))
	}

	return kept
}

func file(out, errOut io.Writer, queuePath, tag, pullRequest string, dedupe bool) error {
	queue, err := LoadQueue(queuePath)
	if err != nil {
		return err
	}

	if len(queue) == 0 {
		_, _ = fmt.Fprintf(out, "nothing to file from %s\n", queuePath)

		return nil
	}

	if dedupe {
		queue, err = withoutOpen(out, queue, tag)
		if err != nil {
			return err
		}

		if len(queue) == 0 {
			_, _ = fmt.Fprintf(out, "every ticket in %s is already open\n", queuePath)

			return nil
		}
	}

	document, err := WriteRendered(queue, tag, pullRequest)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(out, "%d ticket(s) -> %s\n", len(queue), TicketsMarkdown)

	plans, err := json.MarshalIndent(planFields(queue), "", "  ")
	if err != nil {
		return fmt.Errorf("encoding the custom fields: %w", err)
	}

	prompt := fmt.Sprintf(filePromptTemplate, len(queue), listID(), ticketStatus(), tag,
		statusRule(), triageScoreField, expectedChangesField, plans, document)

	created, err := createTickets(prompt)
	if err != nil {
		return err
	}

	return report(out, errOut, len(queue), created)
}

// withoutOpen lists the open tickets under tag and drops what they cover. A
// listing that fails files nothing: not knowing what is open is exactly the
// state this filters for.
func withoutOpen(out io.Writer, queue []Finding, tag string) ([]Finding, error) {
	open, err := openTasks(tag)
	if err != nil {
		return nil, fmt.Errorf("%w: the open %s tickets could not be listed: %w", ErrNotFiled, tag, err)
	}

	return dropAlreadyOpen(out, queue, open), nil
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
func report(out, errOut io.Writer, wanted int, created []Ticket) error {
	filed, failed := split(created)

	err := saveRecord(created)
	if err != nil {
		return err
	}

	for _, ticket := range filed {
		_, _ = fmt.Fprintf(out, "filed %-12s %s\n",
			ticket.ID, textutil.Truncate(ticket.Title, filedTitleWidth))
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

		_, _ = fmt.Fprintf(errOut, "FAILED %-11s %s — %s\n", "-", textutil.Truncate(title, failedTitleWidth), reason)
	}

	// A count that does not match is a silent drop, which is the whole
	// failure this queue exists to prevent — so it is an error, not a note.
	if len(created) != wanted {
		_, _ = fmt.Fprintf(errOut,
			"MISMATCH: %d ticket(s) in the queue, %d row(s) returned. Nothing is trustworthy here.\n",
			wanted, len(created))

		return ErrNotFiled
	}

	if len(failed) > 0 {
		_, _ = fmt.Fprintf(errOut, "%d of %d ticket(s) were NOT created.\n", len(failed), wanted)

		return ErrNotFiled
	}

	warnUnfilled(errOut, filed)
	warnMisplaced(errOut, filed)

	_, _ = fmt.Fprintf(out, "%d ticket(s) filed; record in %s\n", len(filed), FiledRecord)

	return nil
}

// warnMisplaced names tickets the filing turn left outside the backlog. Not
// an error — the task exists — but naming it is the only thing between a
// misfiled proposal and an unattended run picking it up.
func warnMisplaced(errOut io.Writer, filed []Ticket) {
	var wrong []string

	for _, ticket := range filed {
		if !strings.EqualFold(ticket.Status, ticketStatus()) {
			wrong = append(wrong, ticket.ID+" ("+orUnknown(ticket.Status)+")")
		}
	}

	if len(wrong) == 0 {
		return
	}

	_, _ = fmt.Fprintf(errOut,
		"NOT IN BACKLOG: %s — move them back before a task-loop picks them up.\n",
		strings.Join(wrong, ", "))
}

// warnUnfilled names the tickets whose custom fields were refused. Not an
// error: the task exists and a Ticket short a field halts task-handle rather
// than misleading it, so this asks for a hand-fill instead of failing a merge.
func warnUnfilled(errOut io.Writer, filed []Ticket) {
	var bare []string

	for _, ticket := range filed {
		if !ticket.FieldsSet {
			bare = append(bare, ticket.ID)
		}
	}

	if len(bare) == 0 {
		return
	}

	_, _ = fmt.Fprintf(errOut,
		"custom fields were NOT set on %d ticket(s): %s\n"+
			"  Fill Triage Score and Expected Changes by hand.\n",
		len(bare), strings.Join(bare, " "))
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
	err := os.MkdirAll(filepath.Dir(FiledRecord), dirMode)
	if err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(FiledRecord), err)
	}

	payload, err := json.MarshalIndent(created, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding the filing record: %w", err)
	}

	err = os.WriteFile(FiledRecord, payload, fileMode)
	if err != nil {
		return fmt.Errorf("writing %s: %w", FiledRecord, err)
	}

	return nil
}
