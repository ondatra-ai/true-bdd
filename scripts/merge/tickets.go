package merge

import (
	"os"
	"strconv"
	"sync"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
)

// create files one ClickUp ticket per finding, tagged fix-now.
//
// The clickup package is the single ClickUp interface — scripts/cmd/clickup is
// the same code behind a binary, which is how the fix-queue skill reaches it
// by path — so its four-heading ticket shape stays that skill's contract
// whichever way it is called.
//
// One knob changed in the port: the timeout is CLICKUP_CLAUDE_TIMEOUT, the
// clickup package's own, because the call is in-process now. MERGE_CLICKUP_
// TIMEOUT bounded the subprocess that no longer exists; both defaulted to
// 900s, so only the name moved.
func (r *Run) create(toCreate []clickup.Finding, round int) []clickup.Finding {
	if len(toCreate) == 0 {
		return nil
	}

	queue := r.roundDir(round) + "/ticket-queue.json"
	r.save(queue, toCreate)

	err := clickup.File(os.Stdout, os.Stderr, queue, "fix-now", strconv.Itoa(r.pr))
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
