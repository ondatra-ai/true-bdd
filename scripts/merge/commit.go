package merge

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
)

//nolint:gochecknoglobals // a compiled pattern and a pathspec list; constants in all but syntax.
var (
	// A fenced answer is stripped line by line, so a fence on its own line
	// goes and the message inside it stays.
	fenceLineRE = regexp.MustCompile("(?m)^\\s*```[a-zA-Z]*\\s*$")

	diffExcludes = []string{
		":(exclude)tests/bdd-cli/fixtures/*/cassettes/*",
		":(exclude)docs/doc-universe.html",
	}
)

const (
	defaultGatesTimeout = 3600 * time.Second
	// An unbounded diff is what failed with 'Prompt is too long' on PR #70.
	defaultDiffBudget = 200_000
	messageTimeout    = 600 * time.Second
)

func gatesTimeout() time.Duration { return envDuration("MERGE_GATES_TIMEOUT", defaultGatesTimeout) }
func diffBudget() int             { return envInt("DIFF_BUDGET_BYTES", defaultDiffBudget) }

// firstLine is the commit message's title.
func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")

	return line
}

// staged is the commit-message context, bounded.
//
// An unbounded diff is what failed with 'Prompt is too long' and no diagnostic
// on PR #70.
func (r *Run) staged() string {
	stat := r.gitChecked("diff", "--cached", "--stat")
	body := r.gitChecked("diff", "--cached")
	budget := diffBudget()
	shape := "the complete diff"

	if utf8.RuneCountInString(body) > budget {
		body = r.gitChecked(append([]string{"diff", "--cached", "--", "."}, diffExcludes...)...)
		shape = "the diff with recorded cassettes and doc-universe.html filtered out"
	}

	if utf8.RuneCountInString(body) > budget {
		body = truncate(body, budget)
		shape = "a PREFIX of the diff, truncated — lean on the complete --stat above " +
			"rather than describing this prefix as the whole change"
	}

	recent := r.gitChecked("log", "-5", "--pretty=format:%s%n%n%b%n---")

	return fmt.Sprintf("=== Recent commits (style reference) ===\n%s\n\n"+
		"=== Staged files (complete) ===\n%s\n\n"+
		"=== Diff — this is %s ===\n%s\n", recent, stat, shape, body)
}

// commit commits and pushes what this round's fixes changed.
//
// No decisions of its own: Main has already established that this round
// planned fixes and that they changed the tree, so there is no "nothing to do"
// path in here to hide the reason a run stopped early. It follows that the
// last round never reaches this function — split empties its fix band, and the
// loop leaves first.
//
// Deliberately not the pr-commit skill: that path also runs scan-recordings, a
// local review round, the ledger, doc-universe and memory sync. Those belong to
// a human's commit. A merge round's commit is a fix commit.
func (r *Run) commit() {
	r.gitChecked("add", "-A")
	r.logf("running the gates before committing")

	if r.sh([]string{Gates}, options{timeout: gatesTimeout(), stream: true}).code != 0 {
		r.dief("the gates are red after the fixes, though every fix reported them green.\n" +
			"  Nothing was committed. Run the gates yourself and read the failure.")
	}

	answer, err := claudecli.Run(commitPrompt+"\n\n"+r.staged(), claudecli.Options{
		Role:    "commit-msg",
		Timeout: messageTimeout,
	})
	if err != nil {
		r.dief("could not generate a commit message: %v", err)
	}

	message := strings.TrimSpace(fenceLineRE.ReplaceAllString(answer, ""))
	if message == "" {
		r.dief("the commit message came back empty")
	}

	path := StateDir + "/commit-msg.txt"

	err = os.MkdirAll(StateDir, dirMode)
	if err != nil {
		r.dief("creating %s: %v", StateDir, err)
	}

	err = os.WriteFile(path, []byte(message+"\n"), fileMode)
	if err != nil {
		r.dief("writing %s: %v", path, err)
	}

	r.gitChecked("commit", "-F", path)
	r.gitChecked("push", "origin", "HEAD")
	r.logf("committed and pushed: %s", firstLine(message))
}

func (r *Run) headSHA() string {
	return strings.TrimSpace(
		r.gh("pr", "view", strconv.Itoa(r.pr), "--json", "headRefOid", "--jq", ".headRefOid"))
}

// reviewedSHA is the commit the last REAL CodeRabbit review was posted against.
//
// A non-empty body is the test, because `@coderabbitai approve` posts an
// APPROVED review with body_len=0 while an actual review is kilobytes of
// COMMENTED. Measured on PR #76:
//
//	CHANGES_REQUESTED  9cc2545  body=9138   <- a real review
//	COMMENTED          41f85a5  body=3872   <- also a real review
//	APPROVED           14e327a  body=0      <- `@coderabbitai approve`
//
// Not sufficient on its own, and merge checks reviewedThisRun first: a review
// that found nothing is also body_len=0, so this test alone rejects a clean PR.
// Measured on PR #83, whose only in-scope change was one doc comment —
// CodeRabbit reviewed it, had nothing to say, and this returned "".
//
// Deliberately no `--jq`: `gh api --paginate` applies the filter to each page
// and concatenates, so a filter ending in `last` emits one SHA per page rather
// than one overall.
func (r *Run) reviewedSHA() string {
	reviewed := ""

	for _, review := range r.reviews() {
		if isBot(review.User.Login) && review.Body != "" {
			reviewed = review.CommitID
		}
	}

	return reviewed
}

// checkPushed insists origin already holds this HEAD. It does not make that
// true.
//
// Committing or pushing on the caller's behalf is how a merge run puts work on
// origin that the caller never decided to publish. So this only ever answers
// the question, and a "no" ends the run.
//
// Two questions, and neither goes through `@{u}`. A tracking ref describes
// LOCAL CONFIG, not what origin holds — pr-commit/commit.sh pushes with
// `git push origin HEAD` and no `-u`, so its branches are fully pushed and have
// no upstream at all, and asking `@{u}` refused those branches while saying
// "push it yourself" about a commit origin already had. `git log @{u}..HEAD` is
// no better: it compares against a local ref that only moves on fetch or push,
// so it can be stale in either direction. Comparing HEAD to the PR's own head
// SHA asks GitHub what the pull request is built on, which is the thing that
// actually has to be true.
func (r *Run) checkPushed() {
	if dirty := r.worktreeChanges(); dirty != "" {
		r.dief("the working tree is dirty — commit it yourself before merging:\n%s",
			indent(dirty, "  "))
	}

	local, remote := strings.TrimSpace(r.gitChecked("rev-parse", "HEAD")), r.headSHA()
	if local != remote {
		r.dief("local HEAD is %s but PR #%d is at %s — origin does not have\n"+
			"  what you are on. Push it yourself before merging:\n"+
			"    git push origin HEAD", short(local), r.pr, short(remote))
	}
}
