package merge

import (
	"strconv"
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

	r.answerCreated(toCreate)

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

// dispose runs the three dispositions in order, on this goroutine. create
// before fix because fix is the only one that edits: a stop after it edits
// leaves a dirty tree the next run refuses to start on.
func (r *Run) dispose(
	toFix, toCreate, toIgnore []clickup.Finding, round int,
) ([]clickup.Finding, []clickup.Finding, []clickup.Finding) {
	started := time.Now()
	created := r.create(toCreate, round)

	report.Leaf("create tickets", started, "tickets", len(created))

	started = time.Now()
	ignored := r.ignore(toIgnore, round)

	report.Leaf("ignore", started, "findings", len(ignored))

	started = time.Now()
	fixed := r.fix(toFix)

	report.Leaf("fix", started, "findings", len(toFix))

	return fixed, created, ignored
}
