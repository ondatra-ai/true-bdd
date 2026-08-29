package taskhandle

import (
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/config"
	"github.com/ondatra-ai/true-bdd/scripts/report"
)

// The three kinds a Spec finding can be. Two are recoverable; the third is
// work nobody asked for, which is a refusal however good it is.
const (
	kindMissing = "missing"
	kindWrong   = "wrong"
	kindUnasked = "unasked"
)

// reviewStep holds the branch against the Ticket on the Spec axis. It is the
// ONE step that reaches a skill — code-review has no Go implementation — and
// scripts/.config.json can switch it off, like the merge postmortem.
func (r *Run) reviewStep() error {
	defer report.Open(StepReview.Label(), report.KeySkipped, !r.reviewEnabled)()

	if !r.reviewEnabled {
		r.logf("switched off in %s — skipping", config.Path)
		r.list.mark(StepReview, markNone, "switched off in "+config.Path)

		return nil
	}

	for {
		found, err := r.reviewTurn()
		if err != nil {
			r.list.mark(StepReview, markFail, "the review turn failed")

			return halt(StepReview, err)
		}

		toFix, unasked := classify(found.SpecFindings)

		if len(unasked) > 0 {
			r.list.mark(StepReview, markFail, "work the Ticket did not ask for")

			return decline("the branch does work the Ticket did not ask for: " + summarise(unasked))
		}

		if len(toFix) == 0 {
			r.list.mark(StepReview, r.reviewMarker(), standardsNote(found.StandardsFindings))

			return nil
		}

		err = r.recoverReview(toFix)
		if err != nil {
			return err
		}
	}
}

// recoverReview spends a unit, fixes, and re-commits — the re-commit is not
// optional: merge refuses a dirty worktree, after the squash has landed.
func (r *Run) recoverReview(toFix []Finding) error {
	attempt, spent := r.budget.spend("the review kept finding gaps")
	if spent != nil {
		r.list.mark(StepReview, markFail, "unmet after "+itoa(retries)+" retries")

		return spent
	}

	r.logf("review findings — retry %d of %d: %s", attempt, retries, summarise(toFix))

	err := r.repair(StepReview, "The spec review found:\n"+detail(toFix))
	if err != nil {
		return err
	}

	return r.commitStep()
}

func (r *Run) reviewTurn() (review, error) {
	err := r.requireMandate(StepReview)
	if err != nil {
		return review{}, err
	}

	prompt := fill(reviewPrompt, "{ticket}", r.ticketBrief())

	return turn[review](prompt, reviewTools, reviewSchema, "plan", "task-review",
		envDuration("TASK_HANDLE_REVIEW_TIMEOUT", defaultReviewTimeout))
}

// classify splits Spec findings into what a fix can close and what refuses the
// whole run. Anything with an unrecognised kind is treated as a gap, not
// silently dropped.
func classify(findings []Finding) ([]Finding, []Finding) {
	var toFix, unasked []Finding

	for _, finding := range findings {
		if finding.Kind == kindUnasked {
			unasked = append(unasked, finding)

			continue
		}

		toFix = append(toFix, finding)
	}

	return toFix, unasked
}

func (r *Run) reviewMarker() marker {
	if r.budget.spent() > 0 {
		return markWarn
	}

	return markDone
}

// standardsNote records what blocks nothing. Findings on the Standards axis
// are logged and never acted on — that is the axis split, not an oversight.
func standardsNote(standards []string) string {
	if len(standards) == 0 {
		return ""
	}

	return itoa(len(standards)) + " Standards findings, block nothing"
}

func summarise(findings []Finding) string {
	parts := make([]string, 0, len(findings))
	for _, finding := range findings {
		parts = append(parts, finding.What)
	}

	return strings.Join(parts, "; ")
}

func detail(findings []Finding) string {
	lines := make([]string, 0, len(findings))
	for _, finding := range findings {
		lines = append(lines, "  - ["+finding.Kind+"] "+finding.Where+" — "+finding.What)
	}

	return strings.Join(lines, "\n")
}
