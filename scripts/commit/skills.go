package commit

import (
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/config"
	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
	"github.com/ondatra-ai/true-bdd/scripts/report"
)

// skillTools is what a skill turn may reach for. No Bash beyond the gates the
// skills themselves name: neither of them commits, stages or pushes — this
// program does that, after they have run.
const skillTools = "Read,Edit,Write,Glob,Grep," +
	"Bash(git --no-pager diff *),Bash(git status *),Bash(go run ./scripts/cmd/linters *)"

const defaultSkillTimeout = 1800 * time.Second

func skillTimeout() time.Duration {
	return envDuration("COMMIT_SKILL_TIMEOUT", defaultSkillTimeout)
}

// syncDocUniverse audits the declared documents against docs/doc-universe.*.
// Always `auto`: this program has nobody to ask, so the alternative to
// resolving by the skill's documented rules is not resolving at all.
func (r *Run) syncDocUniverse() {
	defer report.Open("doc universe", report.KeySkipped, !r.docUniverseEnabled)()

	if !r.docUniverseEnabled {
		r.logf("switched off in %s — skipping", config.Path)

		return
	}

	r.runSkill("sync-doc-universe", docUniversePrompt, "sync-doc-universe")
}

// updateMemory folds anything the pending diff made false in CLAUDE.md into
// this commit, before the staging step picks it up.
func (r *Run) updateMemory() {
	defer report.Open("memory", report.KeySkipped, !r.updateMemoryEnabled)()

	if !r.updateMemoryEnabled {
		r.logf("switched off in %s — skipping", config.Path)

		return
	}

	r.runSkill("update-memory", memoryPrompt, "update-memory")
}

// runSkill drives one skill as a headless turn and prints what it decided.
// The report is the whole point of running it here rather than in the session:
// nobody watched it happen, so the record has to survive in the output.
func (r *Run) runSkill(name, prompt, role string) {
	answer, err := claudecli.Run(prompt, claudecli.Options{
		AllowedTools:   skillTools,
		PermissionMode: "acceptEdits",
		Role:           role,
		Timeout:        skillTimeout(),
	})
	if err != nil {
		r.dief("the %s skill could not finish: %v\n"+
			"  Nothing was committed. Run it yourself and read the failure.", name, err)
	}

	decisions := strings.TrimSpace(answer)
	if decisions == "" {
		r.dief("the %s skill reported nothing, so there is no record of what it decided.\n"+
			"  Nothing was committed.", name)
	}

	for _, line := range strings.Split(decisions, "\n") {
		r.logf("%s", line)
	}
}
