package remote

// handledRunsCap bounds the dispatch-dedup set (plan §3.6: prune the
// spike's unbounded handled-run set). Redelivery only matters until a run
// is claimed, so evicting the oldest ids far past that is safe.
const handledRunsCap = 512

// handledRuns is a bounded FIFO set of run ids the agent has already
// handed to its main loop. It dedups poll redelivery of a dispatched run
// until the run is claimed (first event posted) without growing without
// bound over a long-lived remote session.
type handledRuns struct {
	seen  map[string]bool
	order []string
	limit int
}

// newHandledRuns builds a bounded handled-run set.
func newHandledRuns(limit int) *handledRuns {
	return &handledRuns{
		seen:  make(map[string]bool),
		order: make([]string, 0, limit),
		limit: limit,
	}
}

// has reports whether runID has already been handled.
func (h *handledRuns) has(runID string) bool {
	return h.seen[runID]
}

// add records runID, evicting the oldest id when over the cap.
func (h *handledRuns) add(runID string) {
	if h.seen[runID] {
		return
	}

	h.seen[runID] = true
	h.order = append(h.order, runID)

	if len(h.order) > h.limit {
		oldest := h.order[0]
		h.order = h.order[1:]
		delete(h.seen, oldest)
	}
}

// remove forgets runID (used when a dispatch could not be enqueued, so a
// later poll re-delivers it).
func (h *handledRuns) remove(runID string) {
	if !h.seen[runID] {
		return
	}

	delete(h.seen, runID)

	for index, id := range h.order {
		if id == runID {
			h.order = append(h.order[:index], h.order[index+1:]...)

			break
		}
	}
}
