package merge

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
	"github.com/ondatra-ai/true-bdd/scripts/internal/diffctx"
)

// A fenced answer is stripped line by line, so a fence on its own line goes
// and the message inside it stays.
var fenceLineRE = regexp.MustCompile("(?m)^\\s*```[a-zA-Z]*\\s*$")

const (
	defaultGatesTimeout = 3600 * time.Second
	messageTimeout      = 600 * time.Second
)

func gatesTimeout() time.Duration { return envDuration("MERGE_GATES_TIMEOUT", defaultGatesTimeout) }
func diffBudget() int             { return envInt("DIFF_BUDGET_BYTES", diffctx.DefaultBudget) }

// firstLine is the commit message's title.
func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")

	return line
}

// staged is the commit-message context, bounded — see diffctx.
func (r *Run) staged() string {
	recent := r.gitChecked("log", "-5", "--pretty=format:%s%n%n%b%n---")

	return fmt.Sprintf("=== Recent commits (style reference) ===\n%s\n\n%s",
		recent, diffctx.Bounded(r.gitChecked, "Staged files", []string{"--cached"}, diffBudget()))
}

// commit commits and pushes what this round's fixes changed. Not scripts/commit:
// that also runs scan-recordings, doc-universe and memory sync — a merge
// round's commit is a fix commit only.
func (r *Run) commit() {
	r.gitChecked("add", "-A")
	r.logf("running the gates before committing")

	if r.sh(GatesArgv, options{timeout: gatesTimeout(), stream: true}).code != 0 {
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

// reviewedSHA is the last commit with a REAL review — body_len is the test:
// auto-approve is body_len=0 (PR #76, below); so is "nothing found" (PR #83) — check reviewedThisRun first.
//
//	CHANGES_REQUESTED  9cc2545  body=9138   <- a real review
//	COMMENTED          41f85a5  body=3872   <- also a real review
//	APPROVED           14e327a  body=0      <- `@coderabbitai approve`
func (r *Run) reviewedSHA() string {
	reviewed := ""

	for _, review := range r.reviews() {
		if isBot(review.User.Login) && review.Body != "" {
			reviewed = review.CommitID
		}
	}

	return reviewed
}

// checkPushed insists origin already holds HEAD (does not make it true).
// Not @{u}: scripts/commit pushes with no -u, so branches have no upstream —
// @{u} errors outright; git log @{u}..HEAD can go stale either way.
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
