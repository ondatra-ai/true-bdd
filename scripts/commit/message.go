package commit

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"

	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
	"github.com/ondatra-ai/true-bdd/scripts/internal/diffctx"
	"github.com/ondatra-ai/true-bdd/scripts/report"
)

// A fenced answer is stripped line by line, so a fence on its own line goes
// and the message inside it stays.
var fenceLineRE = regexp.MustCompile("(?m)^\\s*```[a-zA-Z]*\\s*$")

// The prompt asks for a `feat/` prefix and a model reading that as a
// conventional-commit subject answers `feat: x`, which git refuses outright.
var (
	commitStyleRE  = regexp.MustCompile(`^([a-z]+)\s*:\s*`)
	unsafeBranchRE = regexp.MustCompile(`[^a-z0-9._/-]+`)
	repeatedDashRE = regexp.MustCompile(`-{2,}`)
)

// sanitizeBranchName turns the answer into something git can accept, rather
// than failing a run that has already passed every gate.
func sanitizeBranchName(answer string) string {
	name := strings.ToLower(firstMeaningfulLine(answer))
	name = commitStyleRE.ReplaceAllString(name, "$1/")
	name = repeatedDashRE.ReplaceAllString(unsafeBranchRE.ReplaceAllString(name, "-"), "-")

	if len(name) > branchNameLimit {
		name = name[:branchNameLimit]
	}

	// Truncation and substitution both leave edges git rejects in a ref.
	return strings.Trim(name, "-/.")
}

const (
	defaultMessageTimeout = 600 * time.Second
	branchNameLimit       = 60

	// styleCommits is how much history a written message is shown for style.
	styleCommits = 5
)

func messageTimeout() time.Duration {
	return envDuration("COMMIT_MESSAGE_TIMEOUT", defaultMessageTimeout)
}

func diffBudget() int { return envInt("DIFF_BUDGET_BYTES", diffctx.DefaultBudget) }

// stage puts everything in the index, including whatever steps 1-4 touched.
func (r *Run) stage() {
	defer report.Open("staging")()

	r.check("staging the worktree", git.StageAll())

	staged, err := git.HasStagedChanges()
	r.check("reading the index", err)

	if !staged {
		r.dief("nothing staged to commit.")
	}

	stat, err := git.StagedStat()
	r.check("reading the staged stat", err)

	r.logf("%s", strings.TrimSpace(stat))
}

// staged is the context a message is written from: the recent commits for
// style, then the complete stat and as much diff body as the budget allows.
func (r *Run) staged() string {
	recent, err := git.RecentCommits(styleCommits)
	r.check("reading recent commits", err)

	staged, err := diffctx.Bounded("Staged files", []string{"--cached"}, diffBudget())
	r.check("reading the staged diff", err)

	return fmt.Sprintf("=== Recent commits (style reference) ===\n%s\n\n%s", recent, staged)
}

// ensureBranch refuses to commit directly to the trunk, cutting a branch named
// after what is already staged instead.
func (r *Run) ensureBranch() {
	current, err := git.CurrentBranch()
	r.check("reading the current branch", err)

	if current != trunk && current != altTrunk {
		return
	}

	defer report.Open("branch")()

	answer := r.ask(branchPrompt, "branch-name")

	name := sanitizeBranchName(answer)
	if name == "" {
		r.dief("the generated branch name came back empty or unusable: %q", answer)
	}

	// git owns the ref rules, so git is asked rather than reimplemented.
	valid, err := git.ValidBranchName(name)
	r.check("checking the branch name", err)

	if !valid {
		r.dief("refusing branch name %q (from %q): git will not take it as a ref", name, answer)
	}

	r.logf("on %s — cutting branch %q for this commit", current, name)
	r.check("cutting the branch", git.CreateBranch(name))
}

// commit writes the message, commits and pushes.
func (r *Run) commit() {
	defer report.Open("commit")()

	message := strings.TrimSpace(fenceLineRE.ReplaceAllString(r.ask(commitPrompt, "commit-msg"), ""))
	if message == "" {
		r.dief("the commit message came back empty")
	}

	path := StateDir + "/commit-msg.txt"
	r.write(path, message+"\n")

	r.check("committing", git.CommitFile(path))
	r.check("pushing", git.PushHead(remote))
	r.logf("committed and pushed: %s", firstMeaningfulLine(message))
}

// ask runs one headless turn over the staged context.
func (r *Run) ask(prompt, role string) string {
	answer, err := claudecli.Run(prompt+"\n\n"+r.staged(), claudecli.Options{
		Role:    role,
		Timeout: messageTimeout(),
	})
	if err != nil {
		r.dief("could not generate the %s: %v", role, err)
	}

	return answer
}

// firstMeaningfulLine is the first line that is neither blank nor a fence.
func firstMeaningfulLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.ReplaceAll(line, "\r", ""))
		if line != "" && !fenceLineRE.MatchString(line) {
			return line
		}
	}

	return ""
}
