package merge

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// merge approves, then lands it.
//
// The loop only exits after a round that committed nothing, so HEAD is the
// commit that round's review was posted against — whichever round that was.
// That is what makes `@coderabbitai approve` honest here: it analyses nothing,
// it resolves threads and stamps whatever HEAD is. On PR #76 it stamped a
// commit no review was ever pinned to, which is why the check below reads the
// review record rather than trusting the sequence.
func (r *Run) merge() {
	r.banner("merge")

	current := r.branchStillOurs()
	r.refuseDirtyMerge()
	r.assertHeadWasReviewed()
	r.approve()
	r.land()
	r.returnToTrunk(current)
}

// branchStillOurs catches the checkout having MOVED during the run.
//
// Not a re-check of an argument — there isn't one. A fix agent has
// Bash(git *), and squashing the wrong branch then deleting it is not
// reversible.
func (r *Run) branchStillOurs() string {
	current := r.currentBranch()
	prBranch := strings.TrimSpace(
		r.gh("pr", "view", strconv.Itoa(r.pr), "--json", "headRefName", "--jq", ".headRefName"))

	if prBranch != current {
		r.dief("PR #%d is on branch '%s', but '%s' is now checked out. "+
			"The branch moved during the run; nothing was merged.", r.pr, prBranch, current)
	}

	return current
}

// refuseDirtyMerge is a merge precondition, not a property of any round.
//
// This function ends in `git checkout main`, which REFUSES on a dirty tree —
// by which point the PR is squashed and the remote branch deleted, stranding
// the checkout on a branch that no longer exists while holding uncommitted
// work. That is PR #76's merge-err.txt verbatim.
func (r *Run) refuseDirtyMerge() {
	if dirty := r.worktreeChanges(); dirty != "" {
		r.dief("the worktree is dirty — refusing to merge:\n%s\n"+
			"  The merge ends in `git checkout main`, which refuses on a dirty tree — after\n"+
			"  the squash and the branch deletion. Nothing was merged.", indent(dirty, "    "))
	}
}

// assertHeadWasReviewed is the thing the whole round structure exists to
// guarantee, checked here rather than inferred from the loop having ended in
// the right place.
//
// `@coderabbitai approve` analyses nothing — it resolves threads and stamps
// whatever HEAD happens to be. On PR #76 that was 14e327a, a commit no review
// had ever been posted against.
func (r *Run) assertHeadWasReviewed() {
	head, reviewed := r.headSHA(), r.reviewedSHA()

	switch {
	case r.reviewedThisRun[head]:
		// A review this run requested and watched arrive. Its body may be
		// empty — with `path_filters` narrowing review to part of the tree, a
		// PR touching nothing in scope draws a genuine review with nothing to
		// say — and that is not the PR #76 failure, which was a stamp on a
		// commit no review had been posted against at all.
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
	r.logf("requesting approval")
	r.sh([]string{"gh", "pr", "comment", strconv.Itoa(r.pr), "--body", "@coderabbitai approve"},
		options{check: true})

	for waited := time.Duration(0); waited < approveBudget; {
		time.Sleep(poll)

		waited += poll

		decision := strings.TrimSpace(r.gh("pr", "view", strconv.Itoa(r.pr),
			"--json", "reviewDecision", "--jq", `.reviewDecision // ""`))
		if decision == "APPROVED" {
			r.logf("approved after %s", waited)

			return
		}
	}

	r.logf("! no approval within %s — trying the merge anyway", approveBudget)
}

// squashArgs is the merge command, with whatever escalation it needs.
func (r *Run) squashArgs(extra ...string) []string {
	return append([]string{
		ghBin, "pr", "merge", strconv.Itoa(r.pr), "--squash", "--delete-branch",
	}, extra...)
}

func (r *Run) land() {
	err := os.MkdirAll(StateDir, dirMode)
	if err != nil {
		r.dief("creating %s: %v", StateDir, err)
	}

	attempt := r.sh(r.squashArgs(), options{})
	if attempt.code == 0 {
		return
	}

	r.logf("ordinary merge refused:")

	for line := range strings.SplitSeq(attempt.stderr, "\n") {
		r.logf("  %s", line)
	}

	r.logf("escalating to --admin (the ruleset grants the admin role a " +
		"pull_request-scoped bypass)")
	r.sh(r.squashArgs("--admin"), options{check: true, stream: true})
}

func (r *Run) returnToTrunk(merged string) {
	trunk := "main"
	if r.git("show-ref", "--verify", "--quiet", "refs/heads/master").code == 0 {
		trunk = "master"
	}

	r.gitChecked("checkout", trunk)
	r.gitChecked("pull", "origin", trunk)
	r.git("branch", "-D", merged)
	r.logf("merged and back on %s", trunk)
}
