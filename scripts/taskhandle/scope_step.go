package taskhandle

import (
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/gates"
	"github.com/ondatra-ai/true-bdd/scripts/merge"
	"github.com/ondatra-ai/true-bdd/scripts/report"
)

// scope holds the real diff against what the Ticket said it would touch. One
// chance to narrow, then the run declines: a change that grew past its Ticket
// is not what anybody approved.
func (r *Run) scope() error {
	defer report.Open(StepScope.Label())()

	globs := parseGlobs(r.detail.ExpectedChanges)

	stray, err := r.stray(globs)
	if err != nil {
		r.list.mark(StepScope, markFail, "could not read the diff")

		return halt(StepScope, err)
	}

	if len(stray) == 0 {
		r.list.mark(StepScope, markDone, r.scopeNote(globs))

		return nil
	}

	r.logf("out of scope: %s", strings.Join(stray, ", "))

	err = r.narrow(globs, stray)
	if err != nil {
		return err
	}

	stray, err = r.stray(globs)
	if err != nil {
		r.list.mark(StepScope, markFail, "could not re-read the diff")

		return halt(StepScope, err)
	}

	if len(stray) > 0 {
		r.list.mark(StepScope, markFail, "still out of scope after narrowing")

		return decline("the change touches " + strings.Join(stray, ", ") +
			", which the Ticket's Expected Changes does not cover, and narrowing did not remove it")
	}

	r.list.mark(StepScope, markWarn, "narrowed back to scope once")

	return nil
}

// stray is every changed path no glob covers. gates.Changed is a two-dot diff
// plus untracked, which is what this step needs: no branch has been cut yet,
// so the work is uncommitted on trunk.
func (r *Run) stray(globs []string) ([]string, error) {
	changed, err := gates.Changed(trunk)
	if err != nil {
		return nil, err //nolint:wrapcheck // the caller wraps it into a halt.
	}

	return outOfScope(changed, globs), nil
}

func (r *Run) narrow(globs, stray []string) error {
	err := r.requireMandate(StepScope)
	if err != nil {
		return err
	}

	prompt := fill(narrowPrompt,
		"{ticket}", r.ticketBrief(),
		"{globs}", strings.Join(globs, "\n"),
		"{stray}", strings.Join(stray, "\n"),
		"{gates}", merge.Gates,
	)

	built, err := r.editTurn(prompt, "task-narrow",
		envDuration("TASK_HANDLE_IMPLEMENT_TIMEOUT", defaultImplementTimeout))
	if err != nil {
		r.list.mark(StepScope, markFail, "the narrowing turn failed")

		return halt(StepScope, err)
	}

	r.logf("narrowed: %s", built.Summary)

	return nil
}

func (r *Run) scopeNote(globs []string) string {
	changed, err := gates.Changed(trunk)
	if err != nil {
		return ""
	}

	return itoa(len(changed)) + " files under " + strings.Join(globs, ", ")
}
