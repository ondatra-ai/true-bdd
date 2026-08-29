package merge

import (
	"strconv"
	"sync"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"github.com/ondatra-ai/true-bdd/scripts/report"
)

// create files one ClickUp ticket per finding, tagged fix-now, via the
// shared clickup package (also scripts/cmd/clickup, run by hand).
// Timeout env var renamed in the port: was MERGE_CLICKUP_TIMEOUT, now CLICKUP_CLAUDE_TIMEOUT (same 900s default).
func (r *Run) create(toCreate []clickup.Finding, round int) []clickup.Finding {
	if len(toCreate) == 0 {
		return nil
	}

	queue := r.roundDir(round) + "/ticket-queue.json"
	r.save(queue, toCreate)

	err := clickup.File(queue, "fix-now", strconv.Itoa(r.pr))
	if err != nil {
		r.dief("filing %d ticket(s) failed: %v\n"+
			"  Threads cannot be answered with a destination that does not exist.",
			len(toCreate), err)
	}

	return toCreate
}

// ignore records a finding and drops it. Written to disk so a score can be
// argued with.
func (r *Run) ignore(toIgnore []clickup.Finding, round int) []clickup.Finding {
	if len(toIgnore) > 0 {
		r.save(r.roundDir(round)+"/ignored.json", toIgnore)
	}

	return toIgnore
}

// disposeConcurrently runs the three dispositions at once.
//
// Only fix touches the worktree, so the other two are free to run beside it.
func (r *Run) disposeConcurrently(
	toFix, toCreate, toIgnore []clickup.Finding, round int,
) ([]clickup.Finding, []clickup.Finding, []clickup.Finding) {
	var (
		fixed, created, ignored []clickup.Finding
		group                   sync.WaitGroup
	)

	group.Add(3) //nolint:mnd // the three dispositions, named on the next three lines.

	// Each times ITSELF rather than opening a node: three concurrent start/end
	// pairs interleave, and a tree parsed from order cannot survive that.
	go func() {
		defer group.Done()

		started := time.Now()
		fixed = r.fix(toFix)

		report.Leaf("fix", started, "findings", len(toFix))
	}()
	go func() {
		defer group.Done()

		started := time.Now()
		created = r.create(toCreate, round)

		report.Leaf("create tickets", started, "tickets", len(created))
	}()
	go func() {
		defer group.Done()

		started := time.Now()
		ignored = r.ignore(toIgnore, round)

		report.Leaf("ignore", started, "findings", len(ignored))
	}()

	group.Wait()

	return fixed, created, ignored
}
