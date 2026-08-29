package taskhandle

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"github.com/ondatra-ai/true-bdd/scripts/report"
	"github.com/ondatra-ai/true-bdd/scripts/state"
)

const processingStatus = "PROCESSING"

// begin binds the Ticket, stamps the mandate and moves it to PROCESSING — in
// that order. The mandate must be stamped BEFORE anything constructs a
// commit.Run or merge.Run: both read it at their own Start.
func (r *Run) begin() error {
	defer report.Open(StepStart.Label())()

	err := state.Set(r.repo, state.TicketKey, r.ticketID)
	if err != nil {
		r.list.mark(StepStart, markFail, "could not bind the Ticket")

		return halt(StepStart, fmt.Errorf("binding the Ticket: %w", err))
	}

	err = state.Set(r.repo, state.MandateKey, r.ticketID)
	if err != nil {
		r.list.mark(StepStart, markFail, "could not stamp the mandate")

		return halt(StepStart, fmt.Errorf("stamping the mandate: %w", err))
	}

	err = clickup.Status(r.ticketID, processingStatus,
		"task-handle took this Ticket.")
	if err != nil {
		r.list.mark(StepStart, markFail, "ClickUp refused "+processingStatus)

		// settleHalt undoes the bind for this step alone: a bound Ticket still
		// in TO DO is one the queue predicate hands out while it is worked.
		return halt(StepStart, err)
	}

	r.list.mark(StepStart, markDone, "bound; `"+processingStatus+"` set")

	return nil
}
