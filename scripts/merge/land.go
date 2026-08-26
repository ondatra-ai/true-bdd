package merge

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
)

// merge approves, then lands it. HEAD is trustworthy because the loop only
// exits after a round that committed nothing — see assertHeadWasReviewed
// for what enforces it (PR #76's stamp-wrong-commit failure).
func (r *Run) merge() {
	r.banner("merge")

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
	prBranch := strings.TrimSpace(
		r.gh("pr", "view", strconv.Itoa(r.pr), "--json", "headRefName", "--jq", ".headRefName"))

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

// requiredCheck is one row of `gh pr checks --required`. bucket is gh's own
// normalisation of the CheckRun and StatusContext states into pass, fail,
// pending, skipping or cancel.
type requiredCheck struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Bucket string `json:"bucket"`
	Link   string `json:"link"`
}

// waitForChecks blocks until no required check gh can SEE is still running —
// one absent from the rollup is invisible here, and left to land()'s refusal.
// Nothing bypasses that any more, so #93's silent merge over IN_PROGRESS is a stop.
func (r *Run) waitForChecks() {
	r.logf("waiting for the required checks")

	for waited := time.Duration(0); ; waited += poll {
		pending := r.pendingChecks(r.requiredChecks())
		if len(pending) == 0 {
			r.logf("every required check gh reports has finished, after %s", waited)

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

// notReportedYet is the substring shared by gh's "no checks reported" and
// "no required checks reported" — the CodeRabbit context before the first
// review is requested. Absent is "not yet", never red.
const notReportedYet = "checks reported on the"

// requiredChecks is what gh reports as required. `gh pr view --json
// statusCheckRollup` carries no isRequired field, so the filtering has to be
// gh's; with --json, gh prints the rollup and exits 0 whatever it holds.
func (r *Run) requiredChecks() []requiredCheck {
	answer := r.sh([]string{
		ghBin, "pr", "checks", strconv.Itoa(r.pr),
		"--required", "--json", "name,state,bucket,link",
	}, options{})

	body := strings.TrimSpace(answer.stdout)

	switch {
	case answer.code != 0 && strings.Contains(answer.stderr, notReportedYet):
		return nil
	case answer.code != 0:
		r.dief("`gh pr checks` failed (%d) — this is not a verdict on the checks:\n%s",
			answer.code, textutil.Truncate(firstNonEmpty(answer.stderr, body), diagnosticLimit))
	case body == "":
		return nil
	}

	var checks []requiredCheck

	err := json.Unmarshal([]byte(body), &checks)
	if err != nil {
		r.dief("could not read `gh pr checks`:\n%s", textutil.Truncate(body, diagnosticLimit))
	}

	return checks
}

// pendingChecks names the checks still running, and stops on any verdict the
// merge cannot survive — including a bucket gh grew after this was written.
func (r *Run) pendingChecks(checks []requiredCheck) []string {
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

// squashArgs is the merge command.
func (r *Run) squashArgs() []string {
	return []string{ghBin, "pr", "merge", strconv.Itoa(r.pr), "--squash", "--delete-branch"}
}

// waitForMergeable holds until GitHub itself calls the PR mergeable. It runs
// AFTER approve() on purpose: BLOCKED is ambiguous while the approval is
// missing, and means only the checks once it is in hand.
func (r *Run) waitForMergeable() {
	r.logf("waiting for GitHub to call #%d mergeable", r.pr)

	for waited := time.Duration(0); ; waited += poll {
		state := strings.TrimSpace(r.gh("pr", "view", strconv.Itoa(r.pr),
			"--json", "mergeStateStatus", "--jq", ".mergeStateStatus"))

		if r.mergeableNow(state) {
			r.logf("%s after %s", state, waited)

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
	attempt := r.sh(r.squashArgs(), options{})
	if attempt.code == 0 {
		return
	}

	r.saveText(StateDir+"/merge-err.txt", attempt.stderr)
	r.dief("the merge was refused and nothing was bypassed:\n%s\n"+
		"  The branch is intact. Read the refusal, fix it, and re-run.",
		indent(attempt.stderr, "    "))
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
