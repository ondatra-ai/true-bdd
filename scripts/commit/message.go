package commit

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
	"github.com/ondatra-ai/true-bdd/scripts/internal/diffctx"
)

// A fenced answer is stripped line by line, so a fence on its own line goes
// and the message inside it stays.
var fenceLineRE = regexp.MustCompile("(?m)^\\s*```[a-zA-Z]*\\s*$")

const (
	defaultMessageTimeout = 600 * time.Second
	branchNameLimit       = 60
)

func messageTimeout() time.Duration {
	return envDuration("COMMIT_MESSAGE_TIMEOUT", defaultMessageTimeout)
}

func diffBudget() int { return envInt("DIFF_BUDGET_BYTES", diffctx.DefaultBudget) }

// stage puts everything in the index, including whatever steps 1-4 touched.
func (r *Run) stage() {
	r.banner("staging")
	r.gitChecked("add", "-A")

	if r.git("diff", "--cached", "--quiet").code == 0 {
		r.dief("nothing staged to commit.")
	}

	r.logf("%s", strings.TrimSpace(r.gitChecked("diff", "--cached", "--stat")))
}

// staged is the context a message is written from: the recent commits for
// style, then the complete stat and as much diff body as the budget allows.
func (r *Run) staged() string {
	recent := r.gitChecked("log", "-5", "--pretty=format:%s%n%n%b%n---")

	return fmt.Sprintf("=== Recent commits (style reference) ===\n%s\n\n%s",
		recent, diffctx.Bounded(r.gitChecked, "Staged files", []string{"--cached"}, diffBudget()))
}

// ensureBranch refuses to commit directly to the trunk, cutting a branch named
// after what is already staged instead.
func (r *Run) ensureBranch() {
	current := strings.TrimSpace(r.gitChecked("branch", "--show-current"))
	if current != trunk && current != altTrunk {
		return
	}

	r.banner("branch")

	name := firstMeaningfulLine(r.ask(branchPrompt, "branch-name"))
	if name == "" {
		r.dief("the generated branch name came back empty")
	}

	// A name starting with '-' would be read by git as a flag rather than as
	// a ref, and quoting does not help with that.
	if strings.HasPrefix(name, "-") {
		r.dief("refusing branch name %q: git would read it as a flag", name)
	}

	if len(name) > branchNameLimit {
		name = name[:branchNameLimit]
	}

	r.logf("on %s — cutting branch %q for this commit", current, name)
	r.gitChecked("checkout", "-b", name)
}

// commit writes the message, commits and pushes.
func (r *Run) commit() {
	r.banner("commit")

	message := strings.TrimSpace(fenceLineRE.ReplaceAllString(r.ask(commitPrompt, "commit-msg"), ""))
	if message == "" {
		r.dief("the commit message came back empty")
	}

	path := StateDir + "/commit-msg.txt"
	r.write(path, message+"\n")

	r.gitChecked("commit", "-F", path)
	// No -u: scripts/merge's checkPushed depends on branches having no
	// upstream, and reads origin's HEAD directly instead.
	r.gitChecked("push", "origin", "HEAD")
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
