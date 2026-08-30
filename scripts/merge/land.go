package merge

import (
	"fmt"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/cli/github"
	"github.com/ondatra-ai/true-bdd/scripts/report"
	"log/slog"
)

// merge approves, then lands it. HEAD is trustworthy because the loop only
// exits after a round that committed nothing — see assertHeadWasReviewed
// for what enforces it (PR #76's stamp-wrong-commit failure).
func (r *Run) merge() {
	defer report.Open("merge")()

	current := r.branchStillOurs()
	r.refuseDirtyMerge()
	r.assertHeadWasReviewed()
	r.waitForChecks()
	r.approve()
	r.waitForMergeable()
	r.land()
	r.returnToTrunk(current)
}

// branchStillOurs catches the checkout having MOVED during the run. Not a
// re-check of an argument — there isn't one; a fix agent has Bash(git *),
// and squashing the wrong branch then deleting it is not reversible.
func (r *Run) branchStillOurs() string {
	current := r.currentBranch()
	prBranch, err := github.PRHeadBranch(r.pr)
	r.check("reading the pull request's branch", err)

	if prBranch != current {
		r.dief("PR #%d is on branch '%s', but '%s' is now checked out. "+
			"The branch moved during the run; nothing was merged.", r.pr, prBranch, current)
	}

	return current
}

// refuseDirtyMerge is a merge precondition, not a property of any round.
// The merge ends in `git checkout main`, which refuses on a dirty tree —
// by then the PR is squashed and the branch deleted (PR #76's merge-err.txt).
func (r *Run) refuseDirtyMerge() {
	if dirty := r.worktreeChanges(); dirty != "" {
		r.dief("the worktree is dirty — refusing to merge:\n%s\n"+
			"  The merge ends in `git checkout main`, which refuses on a dirty tree — after\n"+
			"  the squash and the branch deletion. Nothing was merged.", indent(dirty, "    "))
	}
}

// assertHeadWasReviewed guards what round structure alone can't ensure:
// `@coderabbitai approve` stamps whatever HEAD is without analysing it — on
// PR #76 that stamped 14e327a, a commit no review had been posted against.
func (r *Run) assertHeadWasReviewed() {
	head, reviewed := r.headSHA(), r.reviewedSHA()

	switch {
	case r.reviewedThisRun[head]:
		// A review this run requested and watched arrive; body may be empty
		// (path_filters narrows scope — nothing in scope still gets a genuine
		// review). That's not PR #76's failure — see the doc comment above.
		r.logf("HEAD %s was reviewed during this run", short(head))
	case reviewed == "":
		r.dief("no CodeRabbit review was posted against %s during this run, and no earlier review\n"+
			"  with a body exists on #%d. There is nothing to approve.", short(head), r.pr)
	case reviewed != head:
		r.dief("the last review was posted against %s, but #%d is now at %s.\n"+
			"  Approving would stamp a commit nothing has reviewed — the exact\n"+
			"  PR #76 failure. Nothing was merged; re-run to review %s.",
			short(reviewed), r.pr, short(head), short(head))
	default:
		r.logf("HEAD %s is the commit the last review was posted against", short(head))
	}
}

func (r *Run) approve() {
	started := time.Now()

	r.logf("requesting approval")
	r.check("requesting approval", github.Comment(r.pr, "@coderabbitai approve"))

	for waited := time.Duration(0); waited < approveBudget; waited += poll {
		time.Sleep(poll)

		decision, err := github.ReviewDecision(r.pr)
		r.check("reading the review decision", err)

		if decision == "APPROVED" {
			report.Leaf("approve", started)

			return
		}
	}

	slog.Warn("no approval within the budget — trying the merge anyway", "budget", approveBudget)
	report.Leaf("approve", started, report.KeyStatus, report.StatusWarned)
}

// waitForChecks blocks until no required check gh can SEE is still running —
// one absent from the rollup is invisible here, and left to land()'s refusal.
// Nothing bypasses that any more, so #93's silent merge over IN_PROGRESS is a stop.
func (r *Run) waitForChecks() {
	started := time.Now()

	r.logf("waiting for the required checks")

	for waited := time.Duration(0); ; waited += poll {
		pending := r.pendingChecks(r.requiredChecks())
		if len(pending) == 0 {
			report.Leaf("wait for checks", started)

			return
		}

		if waited >= checksBudget {
			r.dief("still waiting on %s after %s. Nothing was merged and the branch\n"+
				"  is intact — let CI finish, then re-run.",
				strings.Join(pending, ", "), checksBudget)
		}

		r.logf("waiting on %s", strings.Join(pending, ", "))
		time.Sleep(poll)
	}
}

// requiredChecks is what gh reports as required, empty until one has reported.
func (r *Run) requiredChecks() []github.Check {
	checks, err := github.RequiredChecks(r.pr)
	r.check("reading the required checks — this is not a verdict on them", err)

	return checks
}

// pendingChecks names the checks still running, and stops on any verdict the
// merge cannot survive — including a bucket gh grew after this was written.
func (r *Run) pendingChecks(checks []github.Check) []string {
	if len(checks) == 0 {
		return []string{"any required check to report"}
	}

	var pending []string

	for _, check := range checks {
		switch check.Bucket {
		case "pass", "skipping":
		case "pending":
			pending = append(pending, fmt.Sprintf("%s (%s)", check.Name, check.State))
		default:
			r.dief("required check %q is %s — nothing was merged and the branch is intact.\n"+
				"  %s", check.Name, check.State, check.Link)
		}
	}

	return pending
}

// waitForMergeable holds until GitHub itself calls the PR mergeable. It runs
// AFTER approve() on purpose: BLOCKED is ambiguous while the approval is
// missing, and means only the checks once it is in hand.
func (r *Run) waitForMergeable() {
	started := time.Now()

	r.logf("waiting for GitHub to call #%d mergeable", r.pr)

	for waited := time.Duration(0); ; waited += poll {
		state, err := github.MergeState(r.pr)
		r.check("reading the merge state", err)

		if r.mergeableNow(state) {
			report.Leaf("wait for mergeable", started, "state", state)

			return
		}

		if waited >= checksBudget {
			r.dief("GitHub still reports #%d as %s after %s. Nothing was merged and the\n"+
				"  branch is intact.\n  %s", r.pr, state, checksBudget, r.checkSummary())
		}

		r.logf("%s — waiting", state)
		time.Sleep(poll)
	}
}

// mergeableNow reads MergeStateStatus. UNSTABLE merges: the failing checks it
// names are the ones the ruleset does not require. UNKNOWN is not an answer —
// mergeability is computed by a job this very query starts.
func (r *Run) mergeableNow(state string) bool {
	switch state {
	case "CLEAN", "UNSTABLE", "HAS_HOOKS":
		return true
	case "BLOCKED", "UNKNOWN":
		return false
	default:
		r.dief("GitHub reports #%d as %s — nothing was merged and the branch is intact.\n"+
			"  BEHIND wants main merged in, DIRTY a conflict resolved, DRAFT the PR\n"+
			"  marked ready.", r.pr, state)

		return false
	}
}

// checkSummary says where the required checks stand, because MergeStateStatus
// reports that a merge is blocked without ever saying by what — on #97 the
// required `gates` run took 4m24s to register, and until then was absent.
func (r *Run) checkSummary() string {
	checks := r.requiredChecks()
	if len(checks) == 0 {
		return "No required check has reported yet."
	}

	seen := make([]string, 0, len(checks))
	for _, check := range checks {
		seen = append(seen, check.Name+"="+check.State)
	}

	return "Reporting: " + strings.Join(seen, ", ") +
		". A required check absent from that list has not registered yet."
}

// land squashes, or stops. The admin bypass is gone: it fired on a pending
// check, a failing one, a conflict and a missing approval identically, and
// merged #93 over a required check still IN_PROGRESS.
func (r *Run) land() {
	refusal, err := github.SquashMerge(r.pr)
	r.check("merging", err)

	if refusal == "" {
		return
	}

	r.saveText(StateDir+"/merge-err.txt", refusal)
	r.dief("the merge was refused and nothing was bypassed:\n%s\n"+
		"  The branch is intact. Read the refusal, fix it, and re-run.",
		indent(refusal, "    "))
}

func (r *Run) returnToTrunk(merged string) {
	trunk := "main"

	master, err := git.LocalBranchExists("master")
	r.check("looking for a master branch", err)

	if master {
		trunk = "master"
	}

	r.check("returning to "+trunk, git.Checkout(trunk))
	r.check("updating "+trunk, git.Pull(remote, trunk))

	// Not checked: a branch gh already deleted with the merge is not an error.
	_ = git.DeleteBranch(merged)

	r.logf("merged and back on %s", trunk)
}
