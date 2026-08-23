package clickup

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

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

// Ticket is one row of the filing turn's answer, and of FiledRecord.
type Ticket struct {
	Title string `json:"title"`
	ID    string `json:"id"`
	URL   string `json:"url"`
	Error string `json:"error"`
}

const filePromptTemplate = `Create ClickUp tasks from the markdown document below, using the ClickUp MCP
tools. Exactly one task per ` + "`## `" + ` heading — %d in total.

For each:
  - list id: %s
  - name: the heading text, without its leading number
  - markdownContent: everything under that heading, verbatim
  - tag: %s

Transcribe. Do not summarise, rewrite, merge, split, reorder or improve any
ticket, and do not create a task that is not in the document. If a task
cannot be created, leave its "id" null and put the error in "error" — do not
retry silently and do not omit the row.

Return ONLY a JSON array, no prose and no code fence:
[{"title": "<heading text>", "id": "<clickup task id or null>", "url": "<url or null>", "error": "<empty if created>"}]

--- BEGIN DOCUMENT ---
%s
--- END DOCUMENT ---
`

// File renders the queue and asks a headless turn to create one ClickUp task
// per heading.
func File(out, errOut io.Writer, queuePath, tag, pullRequest string) error {
	queue, err := LoadQueue(queuePath)
	if err != nil {
		return err
	}

	if len(queue) == 0 {
		_, _ = fmt.Fprintf(out, "nothing to file from %s\n", queuePath)

		return nil
	}

	document, err := WriteRendered(queue, tag, pullRequest)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(out, "%d ticket(s) -> %s\n", len(queue), TicketsMarkdown)

	prompt := fmt.Sprintf(filePromptTemplate, len(queue), listID(), tag, document)

	created, err := createTickets(prompt)
	if err != nil {
		return err
	}

	return report(out, errOut, queue, created)
}

// createTickets runs the filing turn and reads the array back.
func createTickets(prompt string) ([]Ticket, error) {
	answer, err := claudecli.Run(prompt, claudecli.Options{
		AllowedTools: createTools,
		Role:         "clickup",
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
func report(out, errOut io.Writer, queue []Finding, created []Ticket) error {
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
	if len(created) != len(queue) {
		_, _ = fmt.Fprintf(errOut,
			"MISMATCH: %d ticket(s) in the queue, %d row(s) returned. Nothing is trustworthy here.\n",
			len(queue), len(created))

		return ErrNotFiled
	}

	if len(failed) > 0 {
		_, _ = fmt.Fprintf(errOut, "%d of %d ticket(s) were NOT created.\n", len(failed), len(queue))

		return ErrNotFiled
	}

	_, _ = fmt.Fprintf(out, "%d ticket(s) filed; record in %s\n", len(filed), FiledRecord)

	return nil
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
