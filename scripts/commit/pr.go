package commit

import (
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/cli/github"
	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
	"github.com/ondatra-ai/true-bdd/scripts/internal/diffctx"
	"github.com/ondatra-ai/true-bdd/scripts/report"
)

// UpdatePR writes the pull request's title and body from the branch, creates
// or edits it, and returns its URL. Exported because scripts/cmd/pr-update is
// the same step on its own, for a branch already committed.
func (r *Run) UpdatePR() string {
	defer report.Open("pull request")()

	title, body := r.splitAnswer(r.askAboutBranch())
	bodyFile := StateDir + "/pr-body.md"

	r.write(bodyFile, body)

	open, err := github.PRExists()
	r.check("looking for an open pull request", err)

	if open {
		r.check("editing the pull request", github.EditPR(title, bodyFile))
	} else {
		r.check("creating the pull request", github.CreatePR(title, bodyFile))
	}

	url, err := github.PRURL()
	r.check("reading the pull request URL", err)

	return url
}

// askAboutBranch runs one headless turn over the branch's commits and its
// diff against the trunk.
func (r *Run) askAboutBranch() string {
	base := remote + "/" + r.trunkRef()
	commits, err := git.BranchCommits(base)
	r.check("reading the branch's commits", err)

	changed, err := diffctx.Bounded("Files changed", []string{base + "...HEAD"}, diffBudget())
	r.check("reading the branch diff", err)

	context := fmt.Sprintf("=== Commits on this branch (vs %s) ===\n%s\n\n%s",
		base, commits, changed)

	answer, err := claudecli.Run(pullRequestPrompt+"\n\n"+context, claudecli.Options{
		Role:    "pr-content",
		Timeout: messageTimeout(),
	})
	if err != nil {
		r.dief("could not generate the pull request content: %v", err)
	}

	return answer
}

// splitAnswer reads the title off line 1 and the body from line 3 on, which is
// the shape the prompt asks for.
func (r *Run) splitAnswer(answer string) (string, string) {
	lines := strings.Split(strings.TrimSpace(fenceLineRE.ReplaceAllString(answer, "")), "\n")

	title := ""
	if len(lines) > 0 {
		title = strings.TrimSpace(lines[0])
	}

	body := ""
	if len(lines) > 2 { //nolint:mnd // title, blank, then the body.
		body = strings.TrimSpace(strings.Join(lines[2:], "\n"))
	}

	if title == "" || body == "" {
		r.dief("parsed an empty pull request title or body from the answer")
	}

	return title, body + "\n"
}
