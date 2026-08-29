package report

// The attribute keys a writer stamps and Fold reads. A record's MESSAGE is the
// node's name: one source for it, and the text stream stays readable.
const (
	KeyTree       = "tree"
	KeyDurationMs = "duration_ms"
	KeyStatus     = "status"
	KeySkipped    = "skipped"
)

// The two values of KeyTree. A record carrying neither is a leaf when it has a
// duration, and prose the report ignores when it has none.
const (
	TreeStart = "start"
	TreeEnd   = "end"
)

// Status is what became of one node or leaf.
type Status string

const (
	// StatusDone is the absence of anything else.
	StatusDone Status = "done"
	// StatusWarned is a WARN record inside the node.
	StatusWarned Status = "warned"
	// StatusFailed is an ERROR record inside it, which in scripts/ means dief.
	StatusFailed Status = "failed"
	// StatusSkipped is a stage scripts/.config.json switched off.
	StatusSkipped Status = "skipped"
	// StatusKilled is a node that never closed with no ERROR to explain it:
	// os.Exit skips every defer, and SIGKILL skips everything.
	StatusKilled Status = "killed"
)

// The order a run reports the worst thing that happened in.
const (
	rankSkipped = iota
	rankDone
	rankWarned
	rankKilled
	rankFailed
)

// rank places one status in that order.
func (s Status) rank() int {
	switch s {
	case StatusSkipped:
		return rankSkipped
	case StatusDone:
		return rankDone
	case StatusWarned:
		return rankWarned
	case StatusKilled:
		return rankKilled
	case StatusFailed:
		return rankFailed
	default:
		return rankDone
	}
}

// worse returns whichever of the two a reader should see first.
func worse(a, b Status) Status {
	if b.rank() > a.rank() {
		return b
	}

	return a
}
