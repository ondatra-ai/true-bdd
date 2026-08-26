package merge

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
	"github.com/ondatra-ai/true-bdd/scripts/mandate"
)

// historyRoles are the headings this run's own headless turns wrote under.
//
//nolint:gochecknoglobals // a constant list.
var historyRoles = []string{roleMerge, "merge-triage", "merge-fix", "commit-msg", "clickup"}

var stampRE = regexp.MustCompile(`^_(\d{4}-\d{2}-\d{2}T[\d:]+Z)`)

const (
	postmortemTimeout = 1800 * time.Second
	// A history line can carry a whole tool result, far past a default
	// scanner's limit.
	scannerStart = 64 * 1024
	scannerMax   = 16 * 1024 * 1024
)

// postmortemFloor is how long the whole task must take before a clean
// automatic run earns the most expensive turn in the loop — ~9 min on PR #99.
const postmortemFloor = 15 * time.Minute

// noTimings is what the prompt says when nothing measured the run. A raw
// {timings} left in the text would read as an instruction.
const noTimings = "(no timing record was kept for this run)"

// PostmortemOptions is one postmortem's whole input, whether the merge loop
// asks for it or a person does.
type PostmortemOptions struct {
	// PR is the pull request the proposals are filed against.
	PR int
	// HistoryFile is the transcript to read, as a path. Not a lookup:
	// docs/history/hook-state is one repo-global pointer, so with two live
	// sessions PR #99's postmortem read a file named for the other one.
	HistoryFile string
	// Since keeps plain turns stamped at or after it; empty keeps them all.
	Since string
	// Timings is the rendered phase table handed to the model.
	Timings string
	// Floor drops proposals the run's own triage row would not have ticketed.
	Floor int
}

// Postmortem reads one run back and files what it suggests, outside any merge
// loop — scripts/cmd/postmortem is the caller. The loop reaches the same code
// through runPostmortem, so there is one implementation and two callers.
func Postmortem(opts PostmortemOptions) {
	run := &Run{pr: opts.PR, startedAt: opts.Since, floors: Floors{Postmortem: opts.Floor}}
	run.postmortem(opts)
}

// PostmortemFloorNow is the proposal floor a standalone run inherits: the same
// triage row a merge started right now would pick.
func PostmortemFloorNow() int { return floorsNow().Postmortem }

// worthAPostmortem is the gate on the automatic run. Either condition alone is
// enough: a run that needed a human, or a task that took long enough to teach.
func worthAPostmortem(automatic bool, total time.Duration) (bool, string) {
	switch {
	case !automatic:
		return true, "the run did not finish automatically"
	case total > postmortemFloor:
		return true, fmt.Sprintf("the task took %s, over the %s floor",
			total.Round(time.Second), postmortemFloor)
	default:
		return false, fmt.Sprintf("the run finished automatically in %s, under the %s floor",
			total.Round(time.Second), postmortemFloor)
	}
}

// runPostmortem reads the mandate live rather than from Start: task-handle
// revokes it the moment the user interjects, mid-run for a background merge.
// A run that stopped never reaches here — that is scripts/cmd/postmortem's job.
func (r *Run) runPostmortem() {
	r.postmortemOrSkip(mandate.Active("."))
}

// postmortemOrSkip spends the loop's most expensive turn only on a run that
// earned it, and prints the skip when it does not — a postmortem that is
// silently absent reads exactly like one that ran and found nothing.
func (r *Run) postmortemOrSkip(automatic bool) {
	worth, why := worthAPostmortem(automatic, r.timings.total())
	if !worth {
		r.banner("postmortem")
		r.logf("skipped — %s", why)

		return
	}

	stop := r.step("postmortem", 0)
	stop(r.postmortem(PostmortemOptions{
		PR:          r.pr,
		HistoryFile: currentHistoryFile(),
		Since:       r.startedAt,
		Timings:     r.timingReport().Render(),
		Floor:       r.floors.Postmortem,
	}))
}

// postmortem reads the run back and files what it suggests. Changes nothing.
// The returned string is the outcome, for the phase record.
func (r *Run) postmortem(opts PostmortemOptions) string {
	r.banner("postmortem")

	transcript := r.historyExtract(opts.HistoryFile)
	if transcript == "" {
		r.logf("no session history to read — skipping")

		return "no history"
	}

	answer, err := claudecli.Run(
		renderPostmortemPrompt(transcript, opts.Timings),
		claudecli.Options{
			AllowedTools: "Read,Glob,Grep",
			Role:         "merge-postmortem",
			Timeout:      postmortemTimeout,
		})
	if err != nil {
		r.logf("! postmortem failed: %v", err)

		return "failed"
	}

	proposals := r.abovePostmortemFloor(parseJSONArrayInto[clickup.Finding](r, answer, "postmortem"))
	if len(proposals) == 0 {
		r.logf("the postmortem proposed nothing above the floor")

		return "nothing above the floor"
	}

	r.fileProposals(proposals)

	return fmt.Sprintf("%d proposed", len(proposals))
}

// fileProposals queues the proposals and proves the run wrote nothing else.
// The postmortem reads; if the worktree moved, something wrote where nothing
// should have, and that must not be left for the next branch to discover.
func (r *Run) fileProposals(proposals []clickup.Finding) {
	queue := StateDir + "/postmortem.json"
	r.save(queue, proposals)

	err := clickup.FileDeduped(os.Stdout, os.Stderr, queue, "merge-improvements", strconv.Itoa(r.pr))
	if err != nil {
		r.logf("! filing the postmortem's proposals failed: %v", err)
	}

	if dirty := r.worktreeChanges(); dirty != "" {
		r.dief("the postmortem left changes in the worktree, which it must never do:\n%s",
			indent(dirty, "  "))
	}

	r.logf("worktree clean")
}

// renderPostmortemPrompt substitutes the transcript and the timing table.
func renderPostmortemPrompt(transcript, timings string) string {
	if strings.TrimSpace(timings) == "" {
		timings = noTimings
	}

	return strings.NewReplacer(
		"{transcript}", transcript,
		"{timings}", timings,
	).Replace(postmortemPrompt)
}

// abovePostmortemFloor drops proposals the run's triage row would not have
// ticketed. Under a mandate the floor is 9, so it will usually stay silent —
// which is the point: it is the only step that notices the loop degrading.
func (r *Run) abovePostmortemFloor(proposals []clickup.Finding) []clickup.Finding {
	var kept []clickup.Finding

	for _, proposal := range proposals {
		if proposal.Score >= r.floors.Postmortem {
			kept = append(kept, proposal)
		}
	}

	return kept
}

// historyExtract is the merge-related part of a session's history file. Never
// handed whole (it runs to tens of MB): CLAUDE_HISTORY_ROLE labels every
// headless turn, so merge turns are addressable by heading.
func (r *Run) historyExtract(path string) string {
	if path == "" {
		return ""
	}

	handle, err := os.Open(path) //nolint:gosec // the caller names the file it wants read.
	if err != nil {
		return ""
	}

	defer handle.Close() //nolint:errcheck // read-only.

	text := r.keepWantedSections(handle)

	if utf8.RuneCountInString(text) > historyBudgetBytes {
		text = "…earlier turns omitted…\n\n" + lastRunes(text, historyBudgetBytes)
	}

	out := StateDir + "/history-extract.md"

	err = os.MkdirAll(StateDir, dirMode)
	if err != nil {
		r.dief("creating %s: %v", StateDir, err)
	}

	err = os.WriteFile(out, []byte(text), fileMode)
	if err != nil {
		r.dief("writing %s: %v", out, err)
	}

	r.logf("history extract: %d bytes from %s -> %s", utf8.RuneCountInString(text), path, out)

	return text
}

// currentHistoryFile is what docs/history/hook-state points at, or "".
func currentHistoryFile() string {
	raw, err := os.ReadFile("docs/history/hook-state")
	if err != nil {
		return ""
	}

	name := strings.TrimSpace(string(raw))
	if name == "" {
		return ""
	}

	path := filepath.Join("docs/history", name)

	_, err = os.Stat(path)
	if err != nil {
		return ""
	}

	return path
}

// keepWantedSections walks the history file and keeps the sections this run
// wrote, plus any plain turn stamped after it started.
func (r *Run) keepWantedSections(handle *os.File) string {
	wanted := map[string]bool{}
	for _, role := range historyRoles {
		wanted["## claude to @"+role] = true
	}

	var (
		kept    strings.Builder
		section strings.Builder
		keep    bool
		started bool
	)

	scanner := bufio.NewScanner(handle)
	scanner.Buffer(make([]byte, 0, scannerStart), scannerMax)

	for scanner.Scan() {
		line := scanner.Text() + "\n"

		if strings.HasPrefix(line, "## ") {
			if keep {
				kept.WriteString(section.String())
			}

			section.Reset()
			section.WriteString(line)

			keep = wanted[strings.TrimSpace(line)]
			started = true

			continue
		}

		if started {
			keep = keep || r.insideRunWindow(line)

			section.WriteString(line)
		}
	}

	if keep {
		kept.WriteString(section.String())
	}

	// A read that stopped early would silently shorten the transcript the
	// postmortem reasons over, and a short transcript reads exactly like a
	// quiet run.
	err := scanner.Err()
	if err != nil {
		r.dief("reading the history file: %v", err)
	}

	return kept.String()
}

// insideRunWindow reports whether a turn's stamp line falls after this run
// started — a plain turn the postmortem should still see. An empty startedAt
// is a standalone run with no window, and keeps everything.
func (r *Run) insideRunWindow(line string) bool {
	if r.startedAt == "" {
		return true
	}

	match := stampRE.FindStringSubmatch(line)

	return match != nil && match[1] >= r.startedAt
}

// lastRunes is the tail of a string, counted the way Python's text[-n:] counts.
func lastRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}

	return string(runes[len(runes)-limit:])
}
