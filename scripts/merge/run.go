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
	fixFloor     = 9
	ticketFloor  = 6
)

// Waiting. CodeRabbit's free tier allows roughly four PR reviews an hour.
const (
	rateLimitSleep = 900 * time.Second
	ackBudget      = 300 * time.Second // the bot answers a request in seconds
	reviewBudget   = 900 * time.Second // then takes minutes to post the review
	poll           = 30 * time.Second
	approveBudget  = 600 * time.Second
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

// Run is one invocation: one pull request, from one checkout.
//
// The fields are set once by Start and never reassigned, because they are
// properties of the run rather than parameters of it. The round number is NOT
// one of them — it changes every iteration, and a value that changes is one a
// reader has to trace, so it is passed to each function that needs it.
type Run struct {
	repo      string
	pr        int
	startedAt string

	// reviewedThisRun holds the commits this run WATCHED a review land
	// against.
	//
	// A review that found nothing has an empty body — the same signature as
	// the `@coderabbitai approve` rubber stamp — so the body test in
	// reviewedSHA alone rejects a legitimately clean PR. Witnessing is what
	// separates them: PR #76's stamp appeared on a commit this loop never
	// asked anyone to review, while a clean review is one we requested and
	// saw arrive. Recorded as it happens, because once `approve` has run the
	// two are indistinguishable in the API.
	reviewedThisRun map[string]bool
}

// Start answers where we are, and whether this is a thing that can be merged
// at all.
//
// Everything that must be true before the first round, answered once and in
// one place, so the loop is only ever the algorithm.
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

	run := &Run{reviewedThisRun: map[string]bool{}}
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

		// This round fixed nothing, so HEAD will not move and the next round
		// would buy a review of a byte-identical tree at a quarter of the
		// hourly quota. On PR #77 all three rounds reviewed the same commit
		// and found nothing to fix in any of them.
		//
		// The converse needs no test here: fix refuses to return having
		// changed no file, so a non-empty toFix always has a commit.
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
	err := os.MkdirAll(filepath.Dir(path), dirMode)
	if err != nil {
		r.dief("creating %s: %v", filepath.Dir(path), err)
	}

	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		r.dief("encoding %s: %v", path, err)
	}

	err = os.WriteFile(path, encoded, fileMode)
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
