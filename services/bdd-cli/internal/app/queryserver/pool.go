package queryserver

import "sync"

// WorkClass classifies a unit of work for the bounded pool (plan §2).
type WorkClass string

// The work classes the bounded pool schedules. Exported so the remote agent
// integrates the SAME pool as the runtime scheduler (finding 6).
const (
	ClassAnswer    WorkClass = "answer"
	ClassDispatch  WorkClass = "dispatch"
	ClassRead      WorkClass = "read"
	ClassInventory WorkClass = "inventory"
	// ClassDoc is the workspace document lane (doc_tree/doc_read/doc_write):
	// its own concurrency bound, independent of the run-mutation lane, so
	// document work never contends with run scheduling for a slot.
	ClassDoc WorkClass = "doc"
	// ClassChat is the workspace chat lane (plan Slice 5, r3 #6): scheduled
	// separately from ClassDispatch so a slow chat turn (a real Claude call)
	// can never occupy the run-mutation slot and stall dispatch/answer.
	ClassChat WorkClass = "chat"
)

// PoolSettings is field-identical to the seam PoolConfig.
type PoolSettings struct {
	MaxReads     int
	MaxInventory int
	// MaxDocs bounds concurrent doc_tree/doc_read/doc_write handling (its own
	// lane, off the run-mutation lane). Zero means unbounded via the default
	// applied by NewWorkerPool (defaultMaxDocs).
	MaxDocs int
	// MaxChat bounds concurrent chat-turn handling (its own lane; a single
	// workspace-wide conversation makes 1 the natural default).
	MaxChat int
}

// Defaults applied when a caller leaves MaxDocs/MaxChat unset (zero) — chosen
// so existing callers that construct PoolSettings with only
// MaxReads/MaxInventory (protocol-era code) keep working unchanged.
const (
	defaultMaxDocs = 4
	defaultMaxChat = 1
)

type unit struct {
	id    string
	class WorkClass
}

// WorkerPool models the query-server's bounded scheduling policy (plan
// §2): mutations serialize one at a time (priority answer > dispatch),
// reads bound by MaxReads, inventory scans by MaxInventory.
type WorkerPool struct {
	mu       sync.Mutex
	settings PoolSettings
	pending  []unit
}

// NewWorkerPool builds a pool with the given bounds. MaxDocs/MaxChat default
// to defaultMaxDocs/defaultMaxChat when left zero (existing protocol-era
// callers construct PoolSettings without them).
func NewWorkerPool(settings PoolSettings) *WorkerPool {
	if settings.MaxDocs <= 0 {
		settings.MaxDocs = defaultMaxDocs
	}

	if settings.MaxChat <= 0 {
		settings.MaxChat = defaultMaxChat
	}

	return &WorkerPool{settings: settings}
}

// Submit records a pending unit.
func (p *WorkerPool) Submit(unitID string, class WorkClass) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.pending = append(p.pending, unit{id: unitID, class: class})
}

// RunnableNow returns the ids runnable now, bound per lane plus one priority mutation, in one switch per class.
//
//nolint:cyclop // see above
func (p *WorkerPool) RunnableNow() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	chosenMutation := p.highestPriorityMutation()

	reads, invs, docs, chats := 0, 0, 0, 0

	var out []string

	for _, item := range p.pending {
		switch item.class {
		case ClassRead:
			if reads < p.settings.MaxReads {
				out = append(out, item.id)
				reads++
			}
		case ClassInventory:
			if invs < p.settings.MaxInventory {
				out = append(out, item.id)
				invs++
			}
		case ClassDoc:
			// Its OWN lane (r3 #6): bounded independently of the run-mutation
			// lane so document work never contends with dispatch/answer.
			if docs < p.settings.MaxDocs {
				out = append(out, item.id)
				docs++
			}
		case ClassChat:
			// A separate lane from ClassDispatch (r3 #6): a slow chat turn
			// (a real Claude call) can never occupy the run-mutation slot.
			if chats < p.settings.MaxChat {
				out = append(out, item.id)
				chats++
			}
		case ClassAnswer, ClassDispatch:
			if item.id == chosenMutation {
				out = append(out, item.id)
			}
		}
	}

	return out
}

// Complete removes a finished unit, freeing its slot.
func (p *WorkerPool) Complete(unitID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	kept := p.pending[:0]
	for _, item := range p.pending {
		if item.id != unitID {
			kept = append(kept, item)
		}
	}

	p.pending = kept
}

// highestPriorityMutation returns the id of the mutation that wins the single
// serialized slot: the first pending answer, else the first pending dispatch.
func (p *WorkerPool) highestPriorityMutation() string {
	for _, item := range p.pending {
		if item.class == ClassAnswer {
			return item.id
		}
	}

	for _, item := range p.pending {
		if item.class == ClassDispatch {
			return item.id
		}
	}

	return ""
}
