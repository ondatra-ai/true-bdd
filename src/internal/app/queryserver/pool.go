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
)

// PoolSettings is field-identical to the seam PoolConfig.
type PoolSettings struct {
	MaxReads     int
	MaxInventory int
}

type unit struct {
	id    string
	class WorkClass
}

// WorkerPool models the query-server's bounded scheduling policy (plan §2):
// mutations are serialized (exactly one at a time, priority answer > dispatch),
// reads are bounded by MaxReads, inventory scans by MaxInventory. It is the
// pure scheduling core — Submit records a pending unit, RunnableNow computes
// the batch that would run concurrently, Complete removes a finished unit.
type WorkerPool struct {
	mu       sync.Mutex
	settings PoolSettings
	pending  []unit
}

// NewWorkerPool builds a pool with the given bounds.
func NewWorkerPool(settings PoolSettings) *WorkerPool {
	return &WorkerPool{settings: settings}
}

// Submit records a pending unit.
func (p *WorkerPool) Submit(unitID string, class WorkClass) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.pending = append(p.pending, unit{id: unitID, class: class})
}

// RunnableNow returns the ids the pool would run concurrently: up to MaxReads
// reads, up to MaxInventory inventory scans, and at most ONE mutation chosen by
// priority (answer over dispatch).
func (p *WorkerPool) RunnableNow() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	chosenMutation := p.highestPriorityMutation()

	reads, invs := 0, 0

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
