package clickup

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/scripts/triage"
)

// documentPromptTemplate files a document that is already written. Render
// builds a review finding's four headings from its fields; a hand-written
// deferral arrives with its own, so this turn only transcribes.
const documentPromptTemplate = `Create ClickUp tasks from the markdown document below, using the ClickUp MCP
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
  - field %s — the "triage_date_millis" integer, passed VERBATIM. It is a
    ClickUp date field, which takes unix MILLISECONDS.
  - field %s — the "triage_commit" string. When the row omits it, leave this
    field alone.

Set no other custom field. Expected Changes, Scope and Good For Agent are a
person's to fill: this document names no file to bound a change against.

Transcribe. Do not summarise, rewrite, merge, split, reorder or improve any
ticket, and do not create a task that is not in the document. If a task
cannot be created, leave its "id" null and put the error in "error" — do not
retry silently and do not omit the row.

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

// FileDocument files a markdown document written by hand — one task per
// `## ` heading, triaged and then transcribed. It is the deferral path for
// anything that is not a review finding, which is what File's queue carries.
func FileDocument(path, tag string) error {
	raw, err := disk.Read(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	sections := splitSections(string(raw))
	if len(sections) == 0 {
		return fmt.Errorf("%w: %s carries no `## ` heading, so there is no ticket in it",
			ErrNotFiled, path)
	}

	slog.Info("Document split into tickets", "count", len(sections), "document", path)

	kept := triageSections(sections, triage.Score)
	if len(kept) == 0 {
		slog.Warn("Nothing was filed: every ticket in the document scored below the floor",
			"document", path, "floor", triage.Floor)

		return nil
	}

	return fileSections(kept, tag)
}

// fileSections files what survived triage.
func fileSections(kept []section, tag string) error {
	prompt, err := documentPrompt(kept, tag, now())
	if err != nil {
		return err
	}

	created, err := createTickets(prompt)
	if err != nil {
		return err
	}

	return report(len(kept), created)
}

// documentPrompt builds the shortened document. The count the turn and report
// are both given is the SURVIVORS' — hand them the original and a deliberate
// drop reads as the silent drop that check exists to catch.
func documentPrompt(kept []section, tag string, taken stamp) (string, error) {
	blocks := make([]string, 0, len(kept))
	queue := make([]Finding, 0, len(kept))

	for _, held := range kept {
		blocks = append(blocks, held.render())
		queue = append(queue, Finding{Title: held.Title, Score: held.Score})
	}

	plans, err := encodeFields(planStamps(queue, taken))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(documentPromptTemplate,
		len(kept), listID(), ticketStatus(), tag, statusRule(),
		triageScoreField, triageDateField, triageCommitField,
		plans, strings.Join(blocks, "\n")), nil
}

// countHeadings counts what the turn is told to create. A `## ` inside a
// fenced block counts too — which report turns into a loud MISMATCH rather
// than a silently extra ticket, so the miscount cannot pass unseen.
func countHeadings(document string) int {
	count := 0

	for line := range strings.SplitSeq(document, "\n") {
		if strings.HasPrefix(line, "## ") {
			count++
		}
	}

	return count
}
