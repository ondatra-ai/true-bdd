// Package git is the `git` command line. It is one of the typed wrappers
// pkg/shell may be reached through; nothing else may spawn git.
//
// What a caller names is an operation — ask what changed, what is staged —
// never an argv. The verbs, the flags and the porcelain formats are this
// package's, because they are git's.
//
// Stopping the run is the caller's business, so every stop is written where
// it happens, over an error.
package git

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Bin is the binary this package spawns.
const Bin = "git"

// diffVerb is the subcommand every scope below is an argument to.
const diffVerb = "diff"

// ------------------------------------------------------------ where we are

// TopLevel is the absolute path of the checkout root.
func TopLevel() (string, error) {
	return output("rev-parse", "--show-toplevel")
}

// --------------------------------------------------------- what has changed

// ChangedAgainst lists the tracked paths this work touches. Two-dot, NOT
// base...HEAD: a caller runs it before the branch is cut, and on the trunk
// three-dot resolves to an empty diff.
func ChangedAgainst(base string) ([]string, error) {
	return lines(diffVerb, "--name-only", base)
}

// StagedPaths lists the hook's pending commit, deletions included.
// Only this query keeps GIT_INDEX_FILE, which may name a partial commit's index.
func StagedPaths() ([]string, error) {
	opt := defaults()
	if index, exists := os.LookupEnv("GIT_INDEX_FILE"); exists {
		opt.Env = opt.Env.Set("GIT_INDEX_FILE=" + index)
	}

	result, err := shell.Run(context.Background(), []string{Bin, diffVerb, "--cached", "--name-only"}, opt)
	if err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("git staged paths: %w", result.Err())
	}

	return nonEmptyLines(result.Stdout), nil
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
	result, err := run("check-ignore", "-q", path)
	if err != nil {
		return false, err
	}

	return result.Code == 0, nil
}

// ------------------------------------------------------------------ spawning

// defaults blank CLAUDECODE rather than stripping it: a child should know it
// is not interactive.
func defaults() shell.Options {
	return shell.Options{Env: Environment().Blank("CLAUDECODE")}
}

// run spawns git. A non-zero exit is Result.Code, never an error: which exits
// are failures is the operation's business.
func run(args ...string) (shell.Result, error) {
	return shell.Run(context.Background(), append([]string{Bin}, args...), defaults())
}

// output is git's trimmed stdout, with a non-zero exit reported as an error.
func output(args ...string) (string, error) {
	result, err := run(args...)
	if err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), result.Err())
	}

	return strings.TrimSpace(result.Stdout), nil
}

// lines is output split into its non-empty lines, which is the shape every
// listing verb's answer is read in.
func lines(args ...string) ([]string, error) {
	out, err := output(args...)
	if err != nil {
		return nil, err
	}

	return nonEmptyLines(out), nil
}

func nonEmptyLines(out string) []string {
	var paths []string

	for line := range strings.SplitSeq(out, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			paths = append(paths, trimmed)
		}
	}

	return paths
}
