package taskhandle

import (
	"strconv"
	"strings"
)

// The four markers. `–` is both "not reached" and "switched off", which is why
// a reached `–` must carry a note saying which.
type marker string

const (
	markDone marker = "✓"
	markWarn marker = "⚠"
	markFail marker = "✗"
	markNone marker = "–"
)

// missingNote stands where a note was required and not given. Printing it is
// better than printing a bare marker the reader cannot act on.
const missingNote = "(no note — task-handle did not record one)"

type row struct {
	mark marker
	note string
}

// checklist is the run's own record. It exists beside the report tree because
// report.Record decodes a fixed field set and drops every other attribute, so
// a note stamped on a report node never survives the fold.
type checklist struct{ rows map[Step]row }

func newChecklist() *checklist { return &checklist{rows: map[Step]row{}} }

// Render is the spec's list: one line per step, marker then note, with the
// unreached tail collapsed — eight `–` lines say nothing.
func (c *checklist) Render() string {
	steps := Steps()

	last := c.lastReached()
	if last == 0 {
		return "- " + collapse(steps)
	}

	lines := make([]string, 0, len(steps))

	for _, step := range steps[:last] {
		lines = append(lines, "- "+step.Label()+" "+c.line(step))
	}

	if last < len(steps) {
		lines = append(lines, "- "+collapse(steps[last:]))
	}

	return strings.Join(lines, "\n")
}

func (c *checklist) mark(step Step, mark marker, note string) {
	c.rows[step] = row{mark: mark, note: note}
}

// line is one row's marker and note. A ✗, a ⚠ or a reached – always carries
// one: a bare marker is not a report.
func (c *checklist) line(step Step) string {
	entry, reached := c.rows[step]
	if !reached {
		return string(markNone) + " not reached"
	}

	note := entry.note
	if note == "" && entry.mark != markDone {
		note = missingNote
	}

	if note == "" {
		return string(entry.mark)
	}

	return string(entry.mark) + " " + note
}

// lastReached is the index one past the last step that ran, so everything
// after it is the tail to collapse.
func (c *checklist) lastReached() int {
	last := 0

	for index, step := range Steps() {
		if _, reached := c.rows[step]; reached {
			last = index + 1
		}
	}

	return last
}

// collapse turns a run of unreached steps into one range line.
func collapse(rest []Step) string {
	if len(rest) == 0 {
		return ""
	}

	span := strconv.Itoa(int(rest[0]))
	if len(rest) > 1 {
		span += "–" + strconv.Itoa(int(rest[len(rest)-1]))
	}

	return span + " " + string(markNone) + " not reached"
}
