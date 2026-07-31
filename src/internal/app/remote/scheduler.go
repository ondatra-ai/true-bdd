package remote

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/ondatra-ai/true-bdd/src/internal/app/queryserver"
)

// Bounded scheduling policy (plan §2, finding 6): mutations are SERIALIZED
// (exactly one at a time, answer > dispatch), reads are bounded to
// maxConcurrentReads, and inventory scans to maxConcurrentInventory. Every
// polled work item flows through the scheduler instead of starting an
// unbounded goroutine per poll.
const (
	maxConcurrentReads     = 4
	maxConcurrentInventory = 1
)

// workScheduler is the runtime concurrency gate built on the query-server
// WorkerPool scheduling model (finding 6). Submit records a unit and starts
// every now-runnable unit on a bounded worker goroutine; when a unit finishes
// it frees its slot and re-pumps, so the number of concurrent handlers can
// never exceed the pool's bounds.
type workScheduler struct {
	pool   *queryserver.WorkerPool
	handle func(ctx context.Context, item workItem)

	mu      sync.Mutex
	items   map[string]scheduledWork
	running map[string]bool
}

type scheduledWork struct {
	ctx  context.Context
	item workItem
}

// newWorkScheduler builds a scheduler that runs `handle` under the pool bounds.
func newWorkScheduler(handle func(context.Context, workItem)) *workScheduler {
	return &workScheduler{
		pool: queryserver.NewWorkerPool(queryserver.PoolSettings{
			MaxReads:     maxConcurrentReads,
			MaxInventory: maxConcurrentInventory,
		}),
		handle:  handle,
		items:   map[string]scheduledWork{},
		running: map[string]bool{},
	}
}

// Submit enqueues a work item and starts any unit that is now runnable.
func (s *workScheduler) Submit(ctx context.Context, item workItem) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items[item.WorkID] = scheduledWork{ctx: ctx, item: item}
	s.pool.Submit(item.WorkID, workClass(item))
	s.pump()
}

// pump starts every runnable-but-not-yet-running unit. The caller holds s.mu.
func (s *workScheduler) pump() {
	for _, workID := range s.pool.RunnableNow() {
		if s.running[workID] {
			continue
		}

		work, ok := s.items[workID]
		if !ok {
			continue
		}

		s.running[workID] = true

		go s.run(workID, work)
	}
}

// run executes one unit, then frees its slot and re-pumps under the lock.
func (s *workScheduler) run(workID string, work scheduledWork) {
	s.handle(work.ctx, work.item)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.pool.Complete(workID)
	delete(s.running, workID)
	delete(s.items, workID)
	s.pump()
}

// workClass maps a work item to its pool class. A `session_detail` query is an
// INVENTORY unit (it takes a filesystem scan) bounded to one at a time; other
// queries are plain reads (plan §2).
func workClass(item workItem) queryserver.WorkClass {
	switch item.Type {
	case workAnswer:
		return queryserver.ClassAnswer
	case workDispatch:
		return queryserver.ClassDispatch
	case workQuery:
		var payload queryPayload
		if json.Unmarshal(item.Payload, &payload) == nil && payload.View == viewSessionDetail {
			return queryserver.ClassInventory
		}

		return queryserver.ClassRead
	default:
		return queryserver.ClassRead
	}
}
