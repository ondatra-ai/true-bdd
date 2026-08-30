// Package github is the `gh` command line, one of the typed wrappers
// pkg/shell may be reached through.
//
// A caller names a pull request and an operation on it. The verbs, the
// `--json` field lists, the `--jq` selectors and gh's own state vocabularies
// are here because they are gh's: scripts/merge was spelling them at 20-odd
// call sites, each with its own hand-written diagnostic, and two packages
// disagreed about whether the URL selector is spelt `-q` or `--jq`.
//
// Nothing here stops a run. A refusal that a person must read comes back as
// text — see SquashMerge — and everything else is an error.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Bin is the binary this package spawns.
const Bin = "gh"

// ErrUnparseable reports that gh's answer was not the JSON it was asked for.
var ErrUnparseable = errors.New("gh did not answer with JSON")

// notReportedYet is the substring shared by gh's "no checks reported" and "no
// required checks reported" — the CodeRabbit context before the first review
// is requested. Absent is "not yet", never red.
const notReportedYet = "checks reported on the"

// Check is one row of `gh pr checks --required`. Bucket is gh's own
// normalisation of the CheckRun and StatusContext states into pass, fail,
// pending, skipping or cancel.
type Check struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Bucket string `json:"bucket"`
	Link   string `json:"link"`
}

// PR is a pull request as its own branch reports it.
type PR struct {
	Number int    `json:"number"`
	State  string `json:"state"`
}

// ------------------------------------------------- the checked-out branch's

// RepoSlug is `owner/name` for the checkout.
func RepoSlug() (string, error) {
	return output("repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
}

// CurrentPR is the pull request open for the checked-out branch. A branch with
// none is an error: gh exits non-zero and has nothing to report.
func CurrentPR() (PR, error) {
	var pull PR

	err := decode(&pull, "pr", "view", "--json", "number,state")

	return pull, err
}

// PRExists reports whether the checked-out branch already has a pull request,
// which is what decides between creating one and editing it.
func PRExists() (bool, error) {
	return succeeds("pr", "view", "--json", "number")
}

// CreatePR opens a pull request for the checked-out branch. The body is a file
// because it is many lines of somebody's prose, not an argument.
func CreatePR(title, bodyFile string) error {
	return silent("pr", "create", "--title", title, "--body-file", bodyFile)
}

// EditPR rewrites an existing pull request's title and body.
func EditPR(title, bodyFile string) error {
	return silent("pr", "edit", "--title", title, "--body-file", bodyFile)
}

// PRURL is the browser URL of the checked-out branch's pull request.
func PRURL() (string, error) {
	return output("pr", "view", "--json", "url", "--jq", ".url")
}

// PRBody is the description of the checked-out branch's pull request.
func PRBody() (string, error) {
	return output("pr", "view", "--json", "body", "--jq", ".body")
}

// SetPRBody replaces that description.
func SetPRBody(body string) error {
	return silent("pr", "edit", "--body", body)
}

// -------------------------------------------------- a pull request by number

// PRHeadSHA is the commit the pull request's branch points at, as GitHub has
// it — which is what a local HEAD is compared against to prove a push landed.
func PRHeadSHA(number int) (string, error) {
	return prField(number, "headRefOid")
}

// PRHeadBranch is the branch the pull request is opened from.
func PRHeadBranch(number int) (string, error) {
	return prField(number, "headRefName")
}

// MergeState is GitHub's MergeStateStatus: CLEAN, UNSTABLE, BLOCKED, BEHIND,
// DIRTY, DRAFT, HAS_HOOKS or UNKNOWN. Reading it is the caller's.
func MergeState(number int) (string, error) {
	return prField(number, "mergeStateStatus")
}

// ReviewDecision is APPROVED, CHANGES_REQUESTED, REVIEW_REQUIRED or empty. The
// jq default is what turns GitHub's null into that empty string.
func ReviewDecision(number int) (string, error) {
	return output("pr", "view", strconv.Itoa(number),
		"--json", "reviewDecision", "--jq", `.reviewDecision // ""`)
}

// Comment posts one comment on a pull request's conversation.
func Comment(number int, body string) error {
	return silent("pr", "comment", strconv.Itoa(number), "--body", body)
}

// RequiredChecks is what gh reports as required, empty when none has reported
// yet. `pr view --json statusCheckRollup` carries no isRequired field, so the
// filtering has to be gh's.
func RequiredChecks(number int) ([]Check, error) {
	result, err := run("pr", "checks", strconv.Itoa(number),
		"--required", "--json", "name,state,bucket,link")
	if err != nil {
		return nil, err
	}

	body := strings.TrimSpace(result.Stdout)

	if result.Code != 0 {
		if strings.Contains(result.Stderr, notReportedYet) {
			return nil, nil
		}

		return nil, fmt.Errorf("gh pr checks %d: %w", number, result.Err())
	}

	if body == "" {
		return nil, nil
	}

	var checks []Check

	err = json.Unmarshal([]byte(body), &checks)
	if err != nil {
		return nil, fmt.Errorf("%w: gh pr checks %d: %w", ErrUnparseable, number, err)
	}

	return checks, nil
}

// SquashMerge squashes the pull request and deletes its branch. A refusal is
// gh's own stderr, returned rather than wrapped: a person reads it verbatim,
// and nothing here bypasses it. Empty means merged.
func SquashMerge(number int) (string, error) {
	result, err := run("pr", "merge", strconv.Itoa(number), "--squash", "--delete-branch")
	if err != nil {
		return "", err
	}

	if result.Code != 0 {
		return result.Stderr, nil
	}

	return "", nil
}

// prField is one `--json` field of one pull request, unwrapped by gh itself.
func prField(number int, field string) (string, error) {
	return output("pr", "view", strconv.Itoa(number), "--json", field, "--jq", "."+field)
}

// ------------------------------------------------------------------ spawning

// defaults blank CLAUDECODE rather than stripping it: a child should know it
// is not interactive.
func defaults() shell.Options {
	return shell.Options{Env: shell.Inherit().Blank("CLAUDECODE")}
}

// run spawns gh. A non-zero exit is Result.Code — RequiredChecks reads one as
// an answer, so it cannot be an error here.
func run(args ...string) (shell.Result, error) {
	return shell.Run(context.Background(), append([]string{Bin}, args...), defaults())
}

// output is gh's trimmed stdout, with a non-zero exit reported as an error.
func output(args ...string) (string, error) {
	result, err := run(args...)
	if err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", fmt.Errorf("gh %s: %w", strings.Join(args, " "), result.Err())
	}

	return strings.TrimSpace(result.Stdout), nil
}

// silent runs a command whose stdout is not the answer.
func silent(args ...string) error {
	_, err := output(args...)

	return err
}

// succeeds reports whether gh exited zero, for the probes whose exit code is
// the whole answer.
func succeeds(args ...string) (bool, error) {
	result, err := run(args...)
	if err != nil {
		return false, err
	}

	return result.Code == 0, nil
}

// decode runs gh and reads its answer into target. An empty answer decodes to
// nothing and is not an error: several gh queries legitimately return none.
func decode(target any, args ...string) error {
	out, err := output(args...)
	if err != nil {
		return err
	}

	if out == "" {
		return nil
	}

	err = json.Unmarshal([]byte(out), target)
	if err != nil {
		return fmt.Errorf("%w: gh %s: %w", ErrUnparseable, strings.Join(args, " "), err)
	}

	return nil
}
