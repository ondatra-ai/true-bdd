package merge

import (
	"bufio"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
	"github.com/ondatra-ai/true-bdd/scripts/state"
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

// postmortem reads the run back and files what it suggests. Changes nothing.
func (r *Run) postmortem() {
	r.banner("postmortem")

	transcript := r.historyExtract()
	if transcript == "" {
		r.logf("no session history to read — skipping")

		return
	}

	answer, err := claudecli.Run(
		strings.ReplaceAll(postmortemPrompt, "{transcript}", transcript),
		claudecli.Options{
			AllowedTools: "Read,Glob,Grep",
			Role:         "merge-postmortem",
			Timeout:      postmortemTimeout,
		})
	if err != nil {
		r.logf("! postmortem failed: %v", err)

		return
	}

	proposals := r.abovePostmortemFloor(parseJSONArrayInto[clickup.Finding](r, answer, "postmortem"))
	if len(proposals) == 0 {
		r.logf("the postmortem proposed nothing above the floor")

		return
	}

	queue := StateDir + "/postmortem.json"
	r.save(queue, proposals)

	err = clickup.FileDeduped(os.Stdout, os.Stderr, queue, "merge-improvements", strconv.Itoa(r.pr))
	if err != nil {
		r.logf("! filing the postmortem's proposals failed: %v", err)
	}

	// The postmortem reads; it never edits. If the worktree moved, something
	// wrote where nothing should have, and that must not be left for the next
	// branch to discover.
	if dirty := r.worktreeChanges(); dirty != "" {
		r.dief("the postmortem left changes in the worktree, which it must never do:\n%s",
			indent(dirty, "  "))
	}

	r.logf("worktree clean")
}

// historyExtract is the merge-related part of this session's history file.
// Never handed whole (it runs to tens of MB): CLAUDE_HISTORY_ROLE labels
// every headless turn, so merge turns are addressable by heading.
func (r *Run) historyExtract() string {
	name := state.Get(".", state.TaskKey)
	if name == "" {
		return ""
	}

	path := state.HistoryFile(".", name)

	handle, err := os.Open(path) //nolint:gosec // the name comes from the repository's own state file.
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

	r.logf("history extract: %d bytes from %s -> %s", utf8.RuneCountInString(text), name, out)

	return text
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
// started — a plain turn the postmortem should still see.
func (r *Run) insideRunWindow(line string) bool {
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
