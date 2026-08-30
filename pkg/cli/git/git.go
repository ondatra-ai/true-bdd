// Package git is the `git` command line. It is one of the typed wrappers
// pkg/shell may be reached through; nothing else may spawn git.
//
// What a caller names is an operation — stage everything, cut a branch, ask
// what changed — never an argv. The verbs, the flags, the pathspec magic and
// the porcelain formats are this package's, because they are git's: three
// copies of `sh()` in scripts/ were each spelling them for themselves, and
// `log -5 --pretty=format:%s%n%n%b%n---` was written out twice verbatim.
//
// Stopping the run is the caller's business. The three copies this replaces
// disagreed about that silently — merge stopped, taskhandle deliberately did
// not — so every stop is now written where it happens, over an error.
package git

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Bin is the binary this package spawns.
const Bin = "git"

// subjectAndBody is how a commit is read for style: subject, blank line, body,
// then a separator no message line can be mistaken for.
const subjectAndBody = "--pretty=format:%s%n%n%b%n---"

// statusPrefix is the width of a porcelain record's `XY ` status columns.
const statusPrefix = 3

// diffVerb is the subcommand every scope below is an argument to.
const diffVerb = "diff"

// ------------------------------------------------------------ where we are

// TopLevel is the absolute path of the checkout root.
func TopLevel() (string, error) {
	return output("rev-parse", "--show-toplevel")
}

// TopLevelIn is TopLevel for a checkout that is not the process's own, under a
// bound of the caller's — the history hook logs a repository it was told
// about, and will not let a wedged git eat its own timeout.
func TopLevelIn(dir string, budget time.Duration) (string, error) {
	return outputWith(bounded(dir, budget), "rev-parse", "--show-toplevel")
}

// HeadSHA is HEAD's full object name.
func HeadSHA() (string, error) {
	return output("rev-parse", "HEAD")
}

// ShortHeadSHA is HEAD's abbreviated object name.
func ShortHeadSHA() (string, error) {
	return output("rev-parse", "--short", "HEAD")
}

// ShortHeadSHAIn is ShortHeadSHA in a named checkout, bounded like TopLevelIn.
func ShortHeadSHAIn(dir string, budget time.Duration) (string, error) {
	return outputWith(bounded(dir, budget), "rev-parse", "--short", "HEAD")
}

// CurrentBranch is the checked-out branch, empty on a detached HEAD.
func CurrentBranch() (string, error) {
	return output("branch", "--show-current")
}

// --------------------------------------------------------- what has changed

// WorktreeChanges is the porcelain listing of uncommitted changes, empty when
// the tree is clean.
func WorktreeChanges() (string, error) {
	return output("status", "--porcelain")
}

// ShortStatus is `status --short`, the listing a person is shown.
func ShortStatus() (string, error) {
	return output("status", "--short")
}

// ChangedPaths is the set of paths git reports as uncommitted. -z, because the
// default format quotes any path with a space or a non-ASCII byte and a quoted
// name matches nothing it is compared against.
func ChangedPaths() (map[string]bool, error) {
	listing, err := output("status", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}

	paths := map[string]bool{}
	records := strings.Split(listing, "\x00")

	for index := 0; index < len(records); index++ {
		record := records[index]
		if record == "" {
			continue
		}

		if record[0] == 'R' {
			index++ // a rename emits its source as a second record
		}

		if len(record) > statusPrefix {
			paths[filepath.Clean(record[statusPrefix:])] = true
		}
	}

	return paths, nil
}

// ChangedAgainst lists the tracked paths this work touches. Two-dot, NOT
// base...HEAD: a caller runs it before the branch is cut, and on the trunk
// three-dot resolves to an empty diff.
func ChangedAgainst(base string) ([]string, error) {
	return lines(diffVerb, "--name-only", base)
}

// UntrackedPaths lists what git does not track and does not ignore.
func UntrackedPaths() ([]string, error) {
	return lines("ls-files", "--others", "--exclude-standard")
}

// ListedFiles is `ls-files -co --exclude-standard`: tracked plus
// untracked-and-not-ignored, so a stray fails before it is ever committed.
func ListedFiles(pathspecs ...string) ([]string, error) {
	return lines(append([]string{"ls-files", "-co", "--exclude-standard"}, pathspecs...)...)
}

// IsIgnored reports whether git ignores the path.
func IsIgnored(path string) (bool, error) {
	return succeeds("check-ignore", "-q", path)
}

// ---------------------------------------------------------------- the index

// StageAll puts the whole worktree in the index, deletions included.
func StageAll() error {
	return silent("add", "-A")
}

// HasStagedChanges reports whether the index differs from HEAD.
func HasStagedChanges() (bool, error) {
	clean, err := succeeds(diffVerb, "--cached", "--quiet")

	return !clean, err
}

// StagedStat is the --stat of what is staged.
func StagedStat() (string, error) {
	return output(diffVerb, "--cached", "--stat")
}

// ------------------------------------------------------------------- diffs

// Diff is the body of a diff scope: `--cached`, `base...HEAD`, or nothing at
// all for the worktree.
func Diff(scope ...string) (string, error) {
	return output(append([]string{diffVerb}, scope...)...)
}

// DiffStat is the same scope's --stat, which stays small enough to send whole.
func DiffStat(scope ...string) (string, error) {
	return output(append(append([]string{diffVerb}, scope...), "--stat")...)
}

// DiffExcluding is Diff with pathspecs left out. The `-- .` is required: an
// exclude pathspec with no positive one beside it matches nothing.
func DiffExcluding(scope []string, excludes ...string) (string, error) {
	args := append(append([]string{diffVerb}, scope...), "--", ".")

	return output(append(args, excludes...)...)
}

// ----------------------------------------------------------------- history

// RecentCommits is the last count commits as subject, body and a separator —
// the style reference a written message is asked to match.
func RecentCommits(count int) (string, error) {
	return output("log", "-"+strconv.Itoa(count), subjectAndBody)
}

// BranchCommits is every commit this branch has that base does not, in the
// same shape.
func BranchCommits(base string) (string, error) {
	return output("log", base+"..HEAD", subjectAndBody)
}

// -------------------------------------------------------- branches and refs

// LocalBranchExists reports whether the branch is in this checkout.
func LocalBranchExists(name string) (bool, error) {
	return refExists("refs/heads/" + name)
}

// RemoteBranchExists reports whether the branch is on the named remote, as of
// the last fetch.
func RemoteBranchExists(remote, name string) (bool, error) {
	return refExists("refs/remotes/" + remote + "/" + name)
}

// ValidBranchName reports whether git will take the name as a ref. git owns
// the rules, so git is asked rather than reimplemented.
func ValidBranchName(name string) (bool, error) {
	return succeeds("check-ref-format", "--branch", name)
}

// CreateBranch cuts a branch at HEAD and checks it out.
func CreateBranch(name string) error {
	return silent("checkout", "-b", name)
}

// Checkout moves to an existing ref.
func Checkout(ref string) error {
	return silent("checkout", ref)
}

// DeleteBranch removes a local branch whether or not it looks merged, which is
// what a squash-merge leaves behind.
func DeleteBranch(name string) error {
	return silent("branch", "-D", name)
}

// Pull fast-forwards a ref from a remote.
func Pull(remote, ref string) error {
	return silent("pull", remote, ref)
}

// ------------------------------------------------------------- committing

// CommitFile commits the index with the message in path, which is a file
// rather than an argument because a message is many lines of somebody's prose.
func CommitFile(path string) error {
	return silent("commit", "-F", path)
}

// PushHead pushes HEAD to the remote's branch of the same name. No -u:
// scripts/merge's checkPushed depends on branches having no upstream.
func PushHead(remote string) error {
	return silent("push", remote, "HEAD")
}

// ------------------------------------------------------------------ spawning

// defaults blank CLAUDECODE rather than stripping it, which is what all three
// predecessors did: a child should know it is not interactive.
func defaults() shell.Options {
	return shell.Options{Env: shell.Inherit().Blank("CLAUDECODE")}
}

// bounded is defaults for a named checkout under a deadline.
func bounded(dir string, budget time.Duration) shell.Options {
	opt := defaults()
	opt.Dir = dir
	opt.Timeout = budget

	return opt
}

// run spawns git. A non-zero exit is Result.Code, never an error: which exits
// are failures is the operation's business, and the probes below say no.
func run(opt shell.Options, args ...string) (shell.Result, error) {
	return shell.Run(context.Background(), append([]string{Bin}, args...), opt)
}

// output is git's trimmed stdout, with a non-zero exit reported as an error.
func output(args ...string) (string, error) {
	return outputWith(defaults(), args...)
}

func outputWith(opt shell.Options, args ...string) (string, error) {
	result, err := run(opt, args...)
	if err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), result.Err())
	}

	return strings.TrimSpace(result.Stdout), nil
}

// silent runs a command whose stdout is not the answer; the answer is whether
// it worked.
func silent(args ...string) error {
	_, err := output(args...)

	return err
}

// succeeds reports whether git exited zero, for the probes whose exit code is
// the whole answer. A command that never started is an error, not a false.
func succeeds(args ...string) (bool, error) {
	result, err := run(defaults(), args...)
	if err != nil {
		return false, err
	}

	return result.Code == 0, nil
}

// refExists reports whether a fully-qualified ref resolves.
func refExists(ref string) (bool, error) {
	return succeeds("show-ref", "--verify", "--quiet", ref)
}

// lines is output split into its non-empty lines, which is the shape every
// listing verb's answer is read in.
func lines(args ...string) ([]string, error) {
	out, err := output(args...)
	if err != nil {
		return nil, err
	}

	var paths []string

	for _, line := range strings.Split(out, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			paths = append(paths, trimmed)
		}
	}

	return paths, nil
}
