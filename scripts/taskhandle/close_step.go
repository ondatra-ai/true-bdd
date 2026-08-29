package taskhandle

import (
	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"github.com/ondatra-ai/true-bdd/scripts/report"
)

// closeStep is the terminal transition: status, comment, unbind, in that
// order, then the mandate goes. Same three effects /task-done has.
func (r *Run) closeStep() error {
	defer report.Open(StepClose.Label())()

	_, err := clickup.CloseBound(r.repo, string(OutcomeDone),
		"merged: "+r.pullRequest+" ("+r.sha+")")
	if err != nil {
		r.list.mark(StepClose, markFail, "could not close the Ticket")

		return halt(StepClose, err)
	}

	r.unmandate()
	r.list.mark(StepClose, markDone, "`"+string(OutcomeDone)+"`")

	return nil
}
