package report

import (
	"errors"
	"fmt"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

var errNoRecords = errors.New("no records for this run")

// preflightName is the row for everything a program said before it opened its
// first node — where a startup refusal lands, having no operation to fail in.
const preflightName = "preflight"

// Run is one process's tree, folded out of the Task's log.
type Run struct {
	ID       string
	Tool     string
	Nodes    []*Node
	Status   Status
	Duration time.Duration
}

// Fold reads path and returns the tree run walked. Filtering to one run is not
// tidiness: merge's fix agents spawn gates as a SEPARATE process into the same
// file, and two processes' interleaved markers corrupt the stack.
func Fold(path, run string) (*Run, error) {
	raw, err := disk.Read(path)
	if err != nil {
		return nil, fmt.Errorf("reading the Task log: %w", err)
	}

	records := decode(raw, run)
	if len(records) == 0 {
		return nil, fmt.Errorf("%w: %s in %s", errNoRecords, run, path)
	}

	return build(records), nil
}

// builder holds the stack of nodes still open, the ones already closed, and
// what was said with none open at all.
type builder struct {
	open  []*Node
	roots []*Node
	top   Node
}

func build(records []Record) *Run {
	first := firstStructured(records)
	last := records[len(records)-1].Time

	builder := &builder{}

	if first > 0 {
		ended := last
		if first < len(records) {
			ended = records[first].Time
		}

		builder.roots = append(builder.roots, preflight(records[:first], ended))
	}

	for _, record := range records[first:] {
		builder.add(record)
	}

	return builder.finish(records)
}

// firstStructured is where the tree begins. Everything before it is preflight.
func firstStructured(records []Record) int {
	for index, record := range records {
		if record.structured() {
			return index
		}
	}

	return len(records)
}

// preflight folds the records before the first marker into one row.
func preflight(records []Record, ended time.Time) *Node {
	node := &Node{
		Name:     preflightName,
		started:  records[0].Time,
		closed:   true,
		Measured: true,
		Duration: ended.Sub(records[0].Time),
	}

	for _, record := range records {
		node.note(record.Level)
	}

	return node
}

// add folds one record into the tree being built.
func (b *builder) add(record Record) {
	b.note(record.Level)

	switch record.Tree {
	case TreeStart:
		b.open = append(b.open, &Node{
			Name: record.Msg, started: record.Time, skipped: record.Skipped,
		})
	case TreeEnd:
		b.close(record)
	default:
		b.leaf(record)
	}
}

// note marks an ERROR on every node still open, because in scripts/ an ERROR
// is dief and the whole stack is about to stop with it. A WARN marks only the
// node it happened in.
func (b *builder) note(level string) {
	// Said with nothing open, it belongs to the run rather than to any
	// operation — which is where a stop after the last one closed lands.
	if len(b.open) == 0 {
		b.top.note(level)

		return
	}

	if level != levelError {
		b.innermost().note(level)

		return
	}

	for _, node := range b.open {
		node.sawError = true
	}
}

func (b *builder) innermost() *Node {
	if len(b.open) == 0 {
		return nil
	}

	return b.open[len(b.open)-1]
}

// close pops the node the end marker belongs to. An unmatched end is dropped:
// a report must survive a log it cannot explain.
func (b *builder) close(record Record) {
	node := b.pop()
	if node == nil {
		return
	}

	node.closed = true
	node.Duration = record.Time.Sub(node.started)
	node.Measured = true

	if record.Status != "" {
		node.reported = record.Status
	}

	b.attach(node)
}

func (b *builder) pop() *Node {
	node := b.innermost()
	if node == nil {
		return nil
	}

	b.open = b.open[:len(b.open)-1]

	return node
}

// leaf attaches a measured record to whatever node is open. Prose carries no
// duration and is ignored here: the terminal wants it, the report does not.
func (b *builder) leaf(record Record) {
	if record.DurationMs == nil {
		return
	}

	b.attach(&Node{
		Name:     record.Msg,
		Duration: time.Duration(*record.DurationMs) * time.Millisecond,
		Measured: true,
		closed:   true,
		reported: leafStatus(record),
	})
}

// leafStatus prefers what the writer said became of the leaf, and falls back
// on the level it said it at.
func leafStatus(record Record) Status {
	switch {
	case record.Status != "":
		return record.Status
	case record.Level == levelError:
		return StatusFailed
	case record.Level == levelWarn:
		return StatusWarned
	default:
		return StatusDone
	}
}

func (b *builder) attach(node *Node) {
	if parent := b.innermost(); parent != nil {
		parent.Children = append(parent.Children, node)

		return
	}

	b.roots = append(b.roots, node)
}

// finish attaches every node still open — innermost first, so each lands
// under its own parent — and settles the statuses.
func (b *builder) finish(records []Record) *Run {
	for len(b.open) > 0 {
		b.attach(b.pop())
	}

	run := &Run{
		ID:       records[0].Run,
		Tool:     records[0].Tool,
		Nodes:    b.roots,
		Status:   StatusDone,
		Duration: records[len(records)-1].Time.Sub(records[0].Time),
	}

	b.top.closed = true
	run.Status = worse(run.Status, b.top.own())

	for _, node := range run.Nodes {
		run.Status = worse(run.Status, node.resolve())
	}

	return run
}
