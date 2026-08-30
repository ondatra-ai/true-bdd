package taskhandle

import (
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/scripts/internal/diffctx"
	"github.com/ondatra-ai/true-bdd/scripts/merge"
	"github.com/ondatra-ai/true-bdd/scripts/report"
)

// work is the two turns the whole command exists to bracket: one plans, one
// implements. Everything else is protocol around them.
func (r *Run) work() error {
	defer report.Open(StepWork.Label())()

	err := r.requireMandate(StepWork)
	if err != nil {
		return err
	}

	drafted, err := r.planTurn()
	if err != nil {
		r.list.mark(StepWork, markFail, "the plan turn failed")

		return halt(StepWork, err)
	}

	if refusal := strings.TrimSpace(drafted.Refusal); refusal != "" {
		r.list.mark(StepWork, markFail, "refused on the merits")

		return decline(refusal)
	}

	r.save("plan.md", drafted.Plan)

	err = r.requireMandate(StepWork)
	if err != nil {
		return err
	}

	return r.implement(drafted)
}

func (r *Run) planTurn() (plan, error) {
	prompt := fill(planPrompt,
		"{ticket}", r.ticketBrief(),
		"{globs}", r.detail.ExpectedChanges,
		"{gates}", merge.Gates,
		"{context}", r.gitContext(),
	)

	return turn[plan](prompt, planTools, planSchema, "plan", "task-plan",
		envDuration("TASK_HANDLE_PLAN_TIMEOUT", defaultPlanTimeout))
}

func (r *Run) implement(drafted plan) error {
	prompt := fill(implementPrompt,
		"{ticket}", r.ticketBrief(),
		"{plan}", drafted.Plan,
		"{globs}", r.detail.ExpectedChanges,
		"{gates}", merge.Gates,
	)

	built, err := r.editTurn(prompt, "task-implement",
		envDuration("TASK_HANDLE_IMPLEMENT_TIMEOUT", defaultImplementTimeout))
	if err != nil {
		r.list.mark(StepWork, markFail, "the implement turn failed")

		return halt(StepWork, err)
	}

	r.logf("implemented: %s", built.Summary)

	// Advisory only. Step 5 is the authority on the gates, and it runs next.
	if !built.GatesGreen {
		r.list.mark(StepWork, markWarn, "the implement turn left the gates red")

		return nil
	}

	r.list.mark(StepWork, markDone, "")

	return nil
}

// editTurn is the one shape implement, fix and narrow all answer in.
func (r *Run) editTurn(prompt, role string, timeout time.Duration) (build, error) {
	return turn[build](prompt, editTools, buildSchema, "acceptEdits", role, timeout)
}

// ticketBrief is the Ticket as a turn reads it: what it is, and its body.
func (r *Run) ticketBrief() string {
	return "**" + r.detail.Name + "** (" + r.detail.URL + ")\n\n" + r.detail.Description
}

// gitContext is the tree as a turn reads it. Failures are swallowed: a missing
// diff is context, never a verdict.
func (r *Run) gitContext() string {
	context, err := diffctx.Bounded("the tree", nil, diffctx.DefaultBudget)
	if err != nil {
		return ""
	}

	return context
}

// save leaves a run's artifact where it can be read after the fact.
func (r *Run) save(name, content string) {
	err := disk.Write(StateDir+"/"+name, []byte(content), disk.Shared)
	if err != nil {
		r.logf("could not write %s: %v", name, err)
	}
}
