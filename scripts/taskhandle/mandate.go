package taskhandle

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/scripts/state"
)

// requireMandate is the gate between LLM steps: a Go process cannot see the
// session, so `history unmandate` is how a person stops a run. Compared
// against the ticket id — a mandate re-stamped for another Ticket revokes this.
func (r *Run) requireMandate(step Step) error {
	held := state.Get(r.repo, state.MandateKey)
	if held == r.ticketID {
		return nil
	}

	r.list.mark(step, markFail, "the mandate was revoked — nothing merged")

	return fmt.Errorf("%w before %s (mandate now %q)",
		errMandateRevoked, step.Label(), held)
}
