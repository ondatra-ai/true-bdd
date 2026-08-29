package merge

import (
	"strconv"
	"sync"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
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

	go func() { defer group.Done(); fixed = r.fix(toFix) }()
	go func() { defer group.Done(); created = r.create(toCreate, round) }()
	go func() { defer group.Done(); ignored = r.ignore(toIgnore, round) }()

	group.Wait()

	return fixed, created, ignored
}
