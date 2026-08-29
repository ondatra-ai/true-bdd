package taskhandle

import (
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/commit"
	"github.com/ondatra-ai/true-bdd/scripts/merge"
	"github.com/ondatra-ai/true-bdd/scripts/report"
)

// commitStep runs the pr-commit workflow: commit.Embed IS
// commit.Start(nil).Main(), in this process, with the render suppressed.
// Red gates are the one stop worth a retry; every other stop halts.
func (r *Run) commitStep() error {
	defer report.Open(StepCommit.Label())()

	for {
		err := commit.Embed()
		if err == nil {
			return r.afterCommit()
		}

		if !commit.IsGatesRed(err) {
			r.list.mark(StepCommit, markFail, "commit stopped: "+firstLine(err.Error()))

			return halt(StepCommit, err)
		}

		attempt, spent := r.budget.spend("the gates stayed red")
		if spent != nil {
			r.list.mark(StepCommit, markFail, "gates red after "+itoa(retries)+" retries")

			return spent
		}

		r.logf("gates red — retry %d of %d", attempt, retries)

		err = r.repair(StepCommit, err.Error())
		if err != nil {
			return err
		}
	}
}

// repair is the shared recovery turn, for both step 5 and step 6.
func (r *Run) repair(step Step, brief string) error {
	err := r.requireMandate(step)
	if err != nil {
		return err
	}

	prompt := fill(fixPrompt,
		"{brief}", brief,
		"{ticket}", r.ticketBrief(),
		"{gates}", merge.Gates,
	)

	built, err := r.editTurn(prompt, "task-fix",
		envDuration("TASK_HANDLE_IMPLEMENT_TIMEOUT", defaultImplementTimeout))
	if err != nil {
		r.list.mark(step, markFail, "the fix turn failed")

		return halt(step, err)
	}

	r.logf("fixed: %s", built.Summary)

	return nil
}

// afterCommit records the PR and puts the Ticket's URL in its body, which
// nothing else does: commit writes the body from the branch and knows nothing
// about a Ticket.
func (r *Run) afterCommit() error {
	url, err := line(ghBin, "pr", "view", "--json", "url", "--jq", ".url")
	if err != nil {
		r.list.mark(StepCommit, markFail, "could not read the pull request")

		return halt(StepCommit, err)
	}

	r.pullRequest = url
	r.linkTicket()

	note := ""
	if spent := r.budget.spent(); spent > 0 {
		note = itoa(spent) + " of " + itoa(retries) + " retries spent"
	}

	r.list.mark(StepCommit, markerFor(r.budget.spent()), note)

	return nil
}

// linkTicket appends the ClickUp URL to the PR body, once. Step 5 can run
// several times, so it checks before it writes.
func (r *Run) linkTicket() {
	body, err := sh(ghBin, "pr", "view", "--json", "body", "--jq", ".body")
	if err != nil {
		r.logf("could not read the pull request body: %v", err)

		return
	}

	if strings.Contains(body, r.detail.URL) {
		return
	}

	_, err = sh(ghBin, "pr", "edit", "--body",
		strings.TrimRight(body, "\n")+"\n\nTicket: "+r.detail.URL)
	if err != nil {
		r.logf("could not link the Ticket into the pull request: %v", err)
	}
}

func markerFor(spent int) marker {
	if spent > 0 {
		return markWarn
	}

	return markDone
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")

	return line
}
