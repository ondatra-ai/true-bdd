package clickup

import (
	"fmt"
	"io"
	"os"
	"strings"
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

Set no custom field. This document was written by hand, so there is no triage
score to carry, and Scope and Good For Agent are a person's to fill.

Transcribe. Do not summarise, rewrite, merge, split, reorder or improve any
ticket, and do not create a task that is not in the document. If a task
cannot be created, leave its "id" null and put the error in "error" — do not
retry silently and do not omit the row.

Return ONLY a JSON array, no prose and no code fence:
[{"title": "<heading text>", "id": "<clickup task id or null>",
  "url": "<url or null>", "status": "<the task's status after creation>",
  "fields_set": true, "error": "<empty if created>"}]

--- BEGIN DOCUMENT ---
%s
--- END DOCUMENT ---
`

// FileDocument files a markdown document written by hand — one task per
// `## ` heading, transcribed. It is the deferral path for anything that is
// not a review finding, which is what File's rendered queue carries.
func FileDocument(out, errOut io.Writer, path, tag string) error {
	raw, err := os.ReadFile(path) //nolint:gosec // the path is an operator's argument.
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	document := string(raw)

	wanted := countHeadings(document)
	if wanted == 0 {
		return fmt.Errorf("%w: %s carries no `## ` heading, so there is no ticket in it",
			ErrNotFiled, path)
	}

	_, _ = fmt.Fprintf(out, "%d ticket(s) from %s\n", wanted, path)

	created, err := createTickets(fmt.Sprintf(documentPromptTemplate,
		wanted, listID(), ticketStatus(), tag, statusRule(), document))
	if err != nil {
		return err
	}

	return report(out, errOut, wanted, created)
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
