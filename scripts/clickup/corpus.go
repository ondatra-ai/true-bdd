package clickup

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// corpusBatch bounds one bodies turn. Every full description in a single
// answer is a ~300 KB reply, which is the reliability risk this splits.
const corpusBatch = 15

// errEmptyCorpus reports a listing that came back with nothing. Not "the list
// is empty": a corpus that failed to load would file every candidate ungated,
// which is the state the gate exists to refuse.
var errEmptyCorpus = errors.New("the corpus is empty")

const corpusPromptTemplate = `Use the ClickUp MCP tools to list EVERY task in list %s — whatever its
status, whatever its tags, closed ones included. Pass includeClosed true.

Page through them: listTasks returns at most 100 per page, so call it with
page 0, then 1, and so on until a page comes back empty. A list this size has
more than one page, and stopping at the first silently loses the rest.

For each task report its "Triage Score" as the NAME of the selected option —
the number ClickUp shows, 1 to 10, and not the option's orderindex or its id —
and its "Triage Date" EXACTLY as the tool gives it, a raw millisecond number
copied digit for digit. Leave either "" when the field is unset.

Return ONLY a JSON array, no prose and no code fence:
[{"id": "<id>", "name": "<name>", "status": "<status>", "url": "<url>",
  "tags": "<comma-separated tag names, or empty>",
  "created": "<ISO-8601 creation date>",
  "triage_score": "<option name, or empty>",
  "triage_date": "<raw ms, or empty>"}]

Transcribe. Do not sort, filter, deduplicate or omit a row — the caller
decides which ones it wants. If there are none, return [].
`

// CorpusRow is one existing ticket, as the similarity turn will read it.
type CorpusRow struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	URL         string `json:"url"`
	Tags        string `json:"tags"`
	Created     string `json:"created"`
	TriageScore string `json:"triage_score"`
	TriageDate  string `json:"triage_date"`
	// Description is not in the listing's answer: listTasks truncates it at
	// 200 characters, so a second turn reads it whole.
	Description string `json:"-"`
}

// corpusPrompt names the list every ticket is read from.
func corpusPrompt() string {
	return fmt.Sprintf(corpusPromptTemplate, listID())
}

// dumpCorpus refreshes CorpusDir from ClickUp and returns what it wrote. Never
// cached: one merge run files fix-now and then the postmortem minutes later,
// so a reused dump would miss the tickets that same run just created.
func dumpCorpus() ([]CorpusRow, error) {
	rows, err := listing[CorpusRow](corpusPrompt(), listTools, "the corpus listing")
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: %s returned no tickets at all", errEmptyCorpus, listID())
	}

	err = fillBodies(rows)
	if err != nil {
		return nil, err
	}

	err = writeCorpus(rows)
	if err != nil {
		return nil, err
	}

	return rows, nil
}

// fillBodies reads each ticket's description in batches, in place.
func fillBodies(rows []CorpusRow) error {
	for start := 0; start < len(rows); start += corpusBatch {
		batch := rows[start:min(start+corpusBatch, len(rows))]

		ids := make([]string, 0, len(batch))
		for _, row := range batch {
			ids = append(ids, labelled(row.ID, row.Name))
		}

		bodies, err := bodiesOf(ids)
		if err != nil {
			return err
		}

		for index := range batch {
			batch[index].Description = bodies[batch[index].ID].Description
		}
	}

	return nil
}

// writeCorpus lands one markdown file per ticket. The tree is cleared first:
// a ticket deleted since the last dump would otherwise stay rankable.
func writeCorpus(rows []CorpusRow) error {
	err := disk.RemoveTree(CorpusDir)
	if err != nil {
		return fmt.Errorf("clearing %s: %w", CorpusDir, err)
	}

	err = disk.Dir(CorpusDir, disk.Shared)
	if err != nil {
		return fmt.Errorf("creating %s: %w", CorpusDir, err)
	}

	for _, row := range rows {
		path := filepath.Join(CorpusDir, row.ID+".md")

		err = disk.Write(path, []byte(row.render()), disk.Shared)
		if err != nil {
			return fmt.Errorf("writing the corpus entry for %s: %w", row.ID, err)
		}
	}

	slog.Info("Corpus dumped", "tickets", len(rows), "dir", CorpusDir)

	return nil
}

// render is one corpus entry: a header the turn can read fields off, then the
// body as ClickUp holds it.
func (r CorpusRow) render() string {
	return fmt.Sprintf(`# %s

id: %s
status: %s
url: %s
tags: %s
created: %s
triage score: %s
triage date: %s

---

%s
`, r.Name, r.ID, r.Status, r.URL, orUnknown(r.Tags), orUnknown(r.Created),
		orUnknown(r.TriageScore), orUnknown(r.TriageDate),
		strings.TrimSpace(r.Description))
}

// index is every id the corpus holds, which is what a returned match is
// checked against — an id that is not here was invented.
func index(rows []CorpusRow) map[string]CorpusRow {
	byID := make(map[string]CorpusRow, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}

	return byID
}
