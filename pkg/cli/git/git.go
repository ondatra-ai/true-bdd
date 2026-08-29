// Package git is the `git` command line. It is one of the typed wrappers
// pkg/shell may be reached through; nothing else may spawn git.
//
// Three shapes cover every call in the tree, and they differ by what the
// caller treats as the answer: Run when the exit code is data, Output when a
// non-zero exit is a failure, Succeeds when the exit code IS the question
// (`--quiet` probes like check-ignore and show-ref --verify).
//
// Stopping the run is the caller's business. The three sh() copies this
// replaces disagreed about that silently — merge stopped, taskhandle
// deliberately did not — so every stop is now written where it happens.
package git

import (
	"context"
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Bin is the binary this package spawns.
const Bin = "git"

// defaults blank CLAUDECODE rather than stripping it, which is what all three
// predecessors did: a child should know it is not interactive.
func defaults() shell.Options {
	return shell.Options{Env: shell.Inherit().Blank("CLAUDECODE")}
}

// Run runs git and hands back the result. A non-zero exit is Result.Code.
func Run(ctx context.Context, args ...string) (shell.Result, error) {
	return RunWith(ctx, defaults(), args...)
}

// RunWith runs git under a caller's options, for the few sites that need a
// working directory or a deadline of their own.
func RunWith(ctx context.Context, opt shell.Options, args ...string) (shell.Result, error) {
	return shell.Run(ctx, append([]string{Bin}, args...), opt)
}

// Output runs git and returns its trimmed stdout, reporting a non-zero exit
// as an error for the callers that treat one that way.
func Output(ctx context.Context, args ...string) (string, error) {
	result, err := Run(ctx, args...)
	if err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), result.Err())
	}

	return strings.TrimSpace(result.Stdout), nil
}

// Succeeds reports whether git exited zero, for the probes whose exit code is
// the whole answer. A command that never started is an error, not a false.
func Succeeds(ctx context.Context, args ...string) (bool, error) {
	result, err := Run(ctx, args...)
	if err != nil {
		return false, err
	}

	return result.Code == 0, nil
}

// TopLevel is the absolute path of the checkout root.
func TopLevel(ctx context.Context) (string, error) {
	return Output(ctx, "rev-parse", "--show-toplevel")
}

// HeadSHA is HEAD's full object name.
func HeadSHA(ctx context.Context) (string, error) {
	return Output(ctx, "rev-parse", "HEAD")
}

// ShortHeadSHA is HEAD's abbreviated object name.
func ShortHeadSHA(ctx context.Context) (string, error) {
	return Output(ctx, "rev-parse", "--short", "HEAD")
}

// CurrentBranch is the checked-out branch, empty on a detached HEAD.
func CurrentBranch(ctx context.Context) (string, error) {
	return Output(ctx, "branch", "--show-current")
}

// StatusPorcelain is the uncommitted-changes listing, empty when clean. Extra
// arguments are appended, which is how a caller asks for -z.
func StatusPorcelain(ctx context.Context, args ...string) (string, error) {
	return Output(ctx, append([]string{"status", "--porcelain"}, args...)...)
}

// IsIgnored reports whether git ignores the path.
func IsIgnored(ctx context.Context, path string) (bool, error) {
	return Succeeds(ctx, "check-ignore", "-q", path)
}

// RefExists reports whether a fully-qualified ref resolves.
func RefExists(ctx context.Context, ref string) (bool, error) {
	return Succeeds(ctx, "show-ref", "--verify", "--quiet", ref)
}
