package report

import "time"

// Node is one operation, sub-operation or leaf of a run. A leaf has no
// children and its duration is the one its writer measured; a node's is the
// distance between its two markers.
type Node struct {
	Name     string
	Status   Status
	Duration time.Duration
	Measured bool
	Children []*Node

	// The bookkeeping Fold resolves Status from, before anything renders.
	started  time.Time
	closed   bool
	skipped  bool
	reported Status
	sawError bool
	sawWarn  bool
}

// resolve settles this node's status and every descendant's, and returns the
// worst thing that happened anywhere beneath it — which is how a run reports
// itself without a parent inheriting a child's handled failure.
func (n *Node) resolve() Status {
	n.Status = n.own()

	worst := n.Status
	for _, child := range n.Children {
		worst = worse(worst, child.resolve())
	}

	return worst
}

// own is this node's own verdict: what its records said, and whether it
// finished at all.
func (n *Node) own() Status {
	switch {
	case n.reported != "":
		return n.reported
	case n.skipped:
		return StatusSkipped
	case !n.closed && n.sawError:
		return StatusFailed
	case !n.closed:
		return StatusKilled
	case n.sawError:
		return StatusFailed
	case n.sawWarn:
		return StatusWarned
	default:
		return StatusDone
	}
}

// note records what one record's level says about this node.
func (n *Node) note(level string) {
	switch level {
	case levelError:
		n.sawError = true
	case levelWarn:
		n.sawWarn = true
	}
}
