package merge

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
)

// fixTools is what a fix agent may reach for. No Bash(python3 *): this repo
// is no longer Python (see lint-layout.sh), and an unused allowance is one
// a fix can wander into.
const fixTools = "Read,Edit,Write,Glob,Grep," +
	"Bash(git *),Bash(go *),Bash(gofmt *),Bash(golangci-lint *)," +
	"Bash(./scripts/*)," +
	"Bash(" + Gates + ")"

const (
	fixBodyLimit       = 6000
	fixTitleLimit      = 70
	fixSummaryLimit    = 150
	agentSummaryLimit  = 300
	silentSummaryLimit = 120
	defaultFixTimeout  = 2400 * time.Second
)

func fixTimeout() time.Duration { return envDuration("MERGE_FIX_TIMEOUT", defaultFixTimeout) }

// fixReport is what a fix agent must return.
type fixReport struct {
	FilesChanged []string `json:"files_changed"`
	GatesGreen   bool     `json:"gates_green"`
	Summary      string   `json:"summary"`
}

// fixSchema is the shape the agent is held to. Spelled out rather than built
// from a map: it is a constant, and a constant that cannot fail to encode
// needs no error path.
const fixSchema = `{"type":"object",` +
	`"required":["files_changed","gates_green","summary"],` +
	`"properties":{` +
	`"files_changed":{"type":"array","items":{"type":"string"}},` +
	`"gates_green":{"type":"boolean"},` +
	`"summary":{"type":"string"}}}`

// fix repairs each finding in turn; a failure stops the run and leaves the
// worktree as-is (no revert, so the evidence and prior fixes survive).
// Sequential: two fix agents sharing a worktree would collide.
func (r *Run) fix(toFix []clickup.Finding) []clickup.Finding {
	if len(toFix) == 0 {
		return nil
	}

	if dirty := r.worktreeChanges(); dirty != "" {
		r.dief("the worktree is dirty before any fix has run:\n%s\n"+
			"  Every path dirty after the fixes is supposed to be a fix. Deal with this\n"+
			"  first, or the two cannot be told apart.", indent(dirty, "    "))
	}

	fixed := make([]clickup.Finding, 0, len(toFix))

	for index, finding := range toFix {
		r.logf("fix %d/%d: %s:%s — %s", index+1, len(toFix),
			finding.File, finding.Line, truncate(finding.Title, fixTitleLimit))

		report := r.runFixAgent(finding)

		r.logf("  -> %s", truncate(report.Summary, fixSummaryLimit))

		finding.FixSummary = report.Summary
		finding.FilesChanged = make([]string, 0, len(report.FilesChanged))

		for _, path := range report.FilesChanged {
			finding.FilesChanged = append(finding.FilesChanged, filepath.Clean(path))
		}

		fixed = append(fixed, finding)
	}

	r.assertFixesLanded(fixed)

	return fixed
}

func (r *Run) runFixAgent(finding clickup.Finding) fixReport {
	prompt := strings.NewReplacer(
		"{file}", finding.File,
		"{line}", finding.Line,
		"{severity}", finding.Severity,
		"{score}", itoa(finding.Score),
		"{title}", finding.Title,
		"{body}", truncate(finding.Body, fixBodyLimit),
		"{gates}", Gates,
	).Replace(fixPrompt)

	raw, err := claudecli.RunJSON(prompt, claudecli.Options{
		AllowedTools:   fixTools,
		PermissionMode: "acceptEdits",
		Schema:         fixSchema,
		Role:           "merge-fix",
		Timeout:        fixTimeout(),
	})

	var report fixReport

	if err == nil && len(raw) > 0 {
		err = json.Unmarshal(raw, &report)
	}

	if err != nil || len(raw) == 0 {
		r.dief("the fix for %s:%s failed: %s\n  finding: %s\n%s",
			finding.File, finding.Line, errorOrNoOutput(err), finding.Title, r.worktreeNote())
	}

	if !report.GatesGreen {
		r.dief("the fix for %s:%s left the gates red.\n  finding: %s\n  agent  : %s\n"+
			"  A finding triage scored %d/10 needs a person.\n%s",
			finding.File, finding.Line, finding.Title, truncate(report.Summary, agentSummaryLimit),
			finding.Score, r.worktreeNote())
	}

	return report
}

// assertFixesLanded checks every file an agent claimed is a file git sees
// changed — a fix that named none, or named one git doesn't see, would let
// resolveConversations tell GitHub "Fixed on this branch" untruthfully.
func (r *Run) assertFixesLanded(fixed []clickup.Finding) {
	var silent []string

	for _, finding := range fixed {
		if len(finding.FilesChanged) == 0 {
			silent = append(silent, "    "+finding.File+":"+finding.Line+" — "+
				truncate(finding.FixSummary, silentSummaryLimit))
		}
	}

	if len(silent) > 0 {
		r.dief("these fixes changed no file:\n%s\n"+
			"  Triage scored them 9-10. Re-score them or fix them by hand, then re-run.",
			strings.Join(silent, "\n"))
	}

	onDisk := r.changedPaths()

	var phantom []string

	for _, finding := range fixed {
		for _, path := range finding.FilesChanged {
			if !onDisk[path] && !slices.Contains(phantom, path) {
				phantom = append(phantom, path)
			}
		}
	}

	if len(phantom) > 0 {
		slices.Sort(phantom)
		r.dief("these paths were reported as edited, and git does not see them changed:\n%s\n"+
			"  The fix agent's report and the worktree disagree; nothing was committed.",
			"    "+strings.Join(phantom, "\n    "))
	}
}

// worktreeNote is what the run is leaving behind, so the reader knows where to
// look.
func (r *Run) worktreeNote() string {
	dirty := r.worktreeChanges()
	if dirty == "" {
		return "  The worktree is unchanged — the agent wrote nothing."
	}

	return "  Nothing was reverted and nothing was committed. The worktree holds this round's\n" +
		"  work so far, including the failed fix:\n" + indent(dirty, "    ")
}

func indent(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		lines[index] = prefix + line
	}

	return strings.Join(lines, "\n")
}

func errorOrNoOutput(err error) string {
	if err == nil {
		return "no structured output"
	}

	return err.Error()
}
