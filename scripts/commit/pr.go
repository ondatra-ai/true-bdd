package commit

import (
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
	"github.com/ondatra-ai/true-bdd/scripts/internal/diffctx"
)

// UpdatePR writes the pull request's title and body from the branch, creates
// or edits it, and returns its URL. Exported because scripts/cmd/pr-update is
// the same step on its own, for a branch already committed.
func (r *Run) UpdatePR() string {
	r.banner("pull request")

	title, body := r.splitAnswer(r.askAboutBranch())

	r.write(StateDir+"/pr-body.md", body)

	if r.sh([]string{ghBin, "pr", "view", "--json", "number"}, false).code == 0 {
		r.gh("pr", "edit", "--title", title, "--body-file", StateDir+"/pr-body.md")
	} else {
		r.gh("pr", "create", "--title", title, "--body-file", StateDir+"/pr-body.md")
	}

	return strings.TrimSpace(r.gh("pr", "view", "--json", "url", "-q", ".url"))
}

// askAboutBranch runs one headless turn over the branch's commits and its
// diff against the trunk.
func (r *Run) askAboutBranch() string {
	base := "origin/" + r.trunkRef()
	commits := r.gitChecked("log", base+"..HEAD", "--pretty=format:%s%n%n%b%n---")

	context := fmt.Sprintf("=== Commits on this branch (vs %s) ===\n%s\n\n%s",
		base, commits,
		diffctx.Bounded(r.gitChecked, "Files changed", []string{base + "...HEAD"}, diffBudget()))

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
