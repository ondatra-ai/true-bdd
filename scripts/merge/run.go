package merge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"github.com/ondatra-ai/true-bdd/scripts/mandate"
)

// Gates is the quality pipeline a fix must leave green.
const Gates = "./.claude/skills/pr-commit/gates.sh"

// StateDir holds every artifact a round produces, so a score can be argued
// with after the fact.
const StateDir = "tmp/merge"

// The round structure. Rounds 1-2 fix and ticket; round 3 only tickets.
const (
	lastFixRound = 2
	lastRound    = 3
)

// Floors is the row Start picks from whether task-handle stamped a mandate.
// Under one nothing is fixed inline — that is what the Ticket is for.
//
//	            drop    ticket   fix inline
//	manual      1-5     6-8      9-10
//	automatic   1-8     9-10     never
type Floors struct {
	Fix        int // fix inline at or above this
	Ticket     int // file a ClickUp Ticket at or above this; below it, drop
	Postmortem int // file a postmortem proposal at or above this
}

const (
	severe   = 9
	worth    = 6
	neverFix = 11
)

//nolint:gochecknoglobals // the two rows of the table above.
var (
	manual    = Floors{Fix: severe, Ticket: worth, Postmortem: worth}
	automatic = Floors{Fix: neverFix, Ticket: severe, Postmortem: severe}
)

// Waiting. CodeRabbit's free tier allows roughly four PR reviews an hour.
const (
	rateLimitSleep = 900 * time.Second
	ackBudget      = 300 * time.Second // the bot answers a request in seconds
	reviewBudget   = 900 * time.Second // then takes minutes to post the review
	poll           = 30 * time.Second
	approveBudget  = 600 * time.Second
	checksBudget   = 900 * time.Second // gates runs ~3 min, CodeRabbit in seconds
)

// historyBudgetBytes caps the transcript the postmortem is handed.
const historyBudgetBytes = 300_000

// The binaries this package drives, and the modes it writes with.
const (
	gitBin   = "git"
	ghBin    = "gh"
	dirMode  = 0o755
	fileMode = 0o600
)

// roleMerge labels this run's own turns in the conversation history.
const roleMerge = "merge"

// Run is one invocation: one pull request, from one checkout. Fields are
// set once by Start and never reassigned — the round number is deliberately
// not one of them; it changes every iteration, so it's passed as a parameter.
type Run struct {
	repo      string
	pr        int
	startedAt string

	// floors is chosen once, from whether task-handle stamped a mandate for
	// the Ticket that is bound right now.
	floors Floors

	// reviewedThisRun holds commits this run WATCHED a review land against —
	// the tiebreaker reviewedSHA's body_len test can't provide on its own
	// (see reviewedSHA), recorded live since post-approve the two look identical.
	reviewedThisRun map[string]bool
}

// Start answers where we are, and whether this can be merged at all —
// every precondition checked once, up front, so the loop is only the
// algorithm.
func Start(args []string) *Run {
	if len(args) > 0 {
		usage("usage: merge — no arguments. The PR comes from the current branch.")
	}

	root := exec.CommandContext(context.Background(), gitBin, "rev-parse", "--show-toplevel")

	top, err := root.Output()
	if err != nil {
		usage("not inside a git repository")
	}

	err = os.Chdir(strings.TrimSpace(string(top)))
	if err != nil {
		usage("cannot enter the repository root: " + err.Error())
	}

	requireTools()

	run := &Run{reviewedThisRun: map[string]bool{}, floors: manual}
	if mandate.Active(".") {
		run.floors = automatic
	}

	branch := run.currentBranch()

	switch branch {
	case "":
		usage("detached HEAD — check out the branch whose PR you want merged")
	case "main", "master":
		usage("on " + branch + " — there is no PR to merge here")
	}

	run.repo = strings.TrimSpace(run.gh("repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner"))
	run.pr = run.openPullRequest(branch)
	run.startedAt = time.Now().UTC().Format("2006-01-02T15:04:05Z")

	run.banner(fmt.Sprintf("merging %s #%d", run.repo, run.pr))

	return run
}

// Main is the whole loop.
func (r *Run) Main() {
	for round := 1; round <= lastRound; round++ {
		r.requestReview(round)

		issues := r.triage(r.readComments(), round)
		toFix, toCreate, toIgnore := r.split(issues, round)

		fixed, created, ignored := r.disposeConcurrently(toFix, toCreate, toIgnore, round)
		r.resolveConversations(fixed, created, ignored)

		// Empty toFix: HEAD won't move, so the next round reviews a byte-identical
		// tree for a quarter of the quota (PR #77: all 3 rounds reviewed one commit).
		// fix() refuses to return having changed no file, so a non-empty toFix always commits.
		if len(toFix) == 0 {
			break
		}

		r.commit()
	}

	r.merge()
	r.postmortem()
}

func (r *Run) currentBranch() string {
	return strings.TrimSpace(r.gitChecked("branch", "--show-current"))
}

// openPullRequest is the PR number for branch, or a stop.
func (r *Run) openPullRequest(branch string) int {
	view := r.sh([]string{ghBin, "pr", "view", "--json", "number,state"}, options{})
	if view.code != 0 {
		usage("no pull request open for '" + branch + "' — push it and open one first")
	}

	var payload struct {
		Number int    `json:"number"`
		State  string `json:"state"`
	}

	err := json.Unmarshal([]byte(view.stdout), &payload)
	if err != nil {
		usage("could not read `gh pr view`: " + err.Error())
	}

	if payload.State != "OPEN" {
		usage(fmt.Sprintf("PR #%d for '%s' is %s, not OPEN", payload.Number, branch, payload.State))
	}

	return payload.Number
}

func requireTools() {
	var missing []string

	for _, tool := range []string{ghBin, gitBin, "claude"} {
		_, err := exec.LookPath(tool)
		if err != nil {
			missing = append(missing, tool)
		}
	}

	if len(missing) > 0 {
		usage("not on PATH: " + strings.Join(missing, ", "))
	}
}

// ---------------------------------------------------------------- reporting

func (r *Run) logf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stdout, "  "+format+"\n", args...)
}

func (r *Run) banner(message string) {
	_, _ = fmt.Fprintf(os.Stdout, "\n══ %s ══\n", message)
}

// die stops the run with a diagnosis. Nothing here is swallowed.
func (r *Run) dief(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\nSTOPPED: "+format+"\n", args...)
	os.Exit(1)
}

// usage stops before the run has a context to report against.
func usage(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}

// ------------------------------------------------------------------- state

// save writes a round's artifact.
func (r *Run) save(path string, payload any) {
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		r.dief("encoding %s: %v", path, err)
	}

	r.saveText(path, string(encoded))
}

// saveText writes an artifact meant to be read rather than parsed — JSON
// would quote a stop's stderr into escaped newlines.
func (r *Run) saveText(path, payload string) {
	err := os.MkdirAll(filepath.Dir(path), dirMode)
	if err != nil {
		r.dief("creating %s: %v", filepath.Dir(path), err)
	}

	err = os.WriteFile(path, []byte(payload), fileMode)
	if err != nil {
		r.dief("writing %s: %v", path, err)
	}
}

func (r *Run) roundDir(round int) string {
	return StateDir + "/round-" + strconv.Itoa(round)
}

// filedRecord is where the ClickUp interface leaves its per-ticket record.
const filedRecord = clickup.FiledRecord

// envDuration reads a per-run override, in seconds.
func envDuration(name string, fallback time.Duration) time.Duration {
	seconds, err := strconv.Atoi(os.Getenv(name))
	if err != nil || seconds <= 0 {
		return fallback
	}

	return time.Duration(seconds) * time.Second
}

// envInt reads a per-run override.
func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil || value <= 0 {
		return fallback
	}

	return value
}
