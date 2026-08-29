package taskhandle

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"github.com/ondatra-ai/true-bdd/scripts/report"
)

// errNotStarted is a Ticket that is not formally ready. It is not a FAILED:
// nothing is written to ClickUp, and a person fills the gaps in.
var errNotStarted = errors.New("the Ticket is not ready to be taken")

func isNotStarted(err error) bool { return errors.Is(err, errNotStarted) }

// check reads the Ticket and holds it against ticket-schema.yaml. Anything
// missing stops the run here, having written nothing at all.
func (r *Run) check() error {
	defer report.Open(StepCheck.Label())()

	detail, err := clickup.Fetch(r.ticketID)
	if err != nil {
		r.list.mark(StepCheck, markFail, "could not read the Ticket")

		return halt(StepCheck, err)
	}

	r.detail = detail

	missing := verify(detail, clickup.Headings())
	if len(missing) > 0 {
		r.list.mark(StepCheck, markFail, strings.Join(missing, "; "))

		return fmt.Errorf("%w:\n  %s", errNotStarted, strings.Join(missing, "\n  "))
	}

	r.list.mark(StepCheck, markDone, "")

	return nil
}
