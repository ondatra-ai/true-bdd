package merge

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/cli/github"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/scripts/gates"
	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
	"github.com/ondatra-ai/true-bdd/scripts/internal/diffctx"
	"github.com/ondatra-ai/true-bdd/scripts/report"
)

// A fenced answer is stripped line by line, so a fence on its own line goes
// and the message inside it stays.
var fenceLineRE = regexp.MustCompile("(?m)^\\s*```[a-zA-Z]*\\s*$")

const (
	messageTimeout = 600 * time.Second

	// styleCommits is how much history a written message is shown for style.
	styleCommits = 5
)

func diffBudget() int { return envInt("DIFF_BUDGET_BYTES", diffctx.DefaultBudget) }

// firstLine is the commit message's title.
func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")

	return line
}

// staged is the commit-message context, bounded — see diffctx.
func (r *Run) staged() string {
	recent, err := git.RecentCommits(styleCommits)
	r.check("reading recent commits", err)

	staged, err := diffctx.Bounded("Staged files", []string{"--cached"}, diffBudget())
	r.check("reading the staged diff", err)

	return fmt.Sprintf("=== Recent commits (style reference) ===\n%s\n\n%s", recent, staged)
}

// commit commits and pushes what this round's fixes changed. Not scripts/commit:
// that also runs scan-recordings, doc-universe and memory sync — a merge
// round's commit is a fix commit only.
func (r *Run) commit() {
	defer report.Open("commit")()

	r.check("staging the worktree", git.StageAll())
	r.logf("running the gates before committing")

	err := r.runGates()
	if err != nil {
		r.dief("the gates are red after the fixes, though every fix reported them green.\n"+
			"  Nothing was committed: %v", err)
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

	err = disk.Write(path, []byte(message+"\n"), disk.Shared)
	if err != nil {
		r.dief("%v", err)
	}

	r.check("committing", git.CommitFile(path))
	r.check("pushing", git.PushHead(remote))
	r.logf("committed and pushed: %s", firstLine(message))
}

// runGates runs the pipeline in THIS process: the table is a package merge
// already imports, so `go run ./scripts/cmd/gates` forked code already linked
// in, and its records landed under a run id scripts/report filters out.
func (r *Run) runGates() error {
	defer report.Open("gates")()

	err := gates.Run(gates.All)
	if err != nil {
		return fmt.Errorf("the pipeline: %w", err)
	}

	return nil
}

func (r *Run) headSHA() string {
	sha, err := github.PRHeadSHA(r.pr)
	r.check("reading the pull request's head", err)

	return sha
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

	local, err := git.HeadSHA()
	r.check("reading HEAD", err)

	pushed := r.headSHA()
	if local != pushed {
		r.dief("local HEAD is %s but PR #%d is at %s — origin does not have\n"+
			"  what you are on. Push it yourself before merging:\n"+
			"    git push origin HEAD", short(local), r.pr, short(pushed))
	}
}
