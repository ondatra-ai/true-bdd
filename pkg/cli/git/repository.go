package git

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Repository is a checkout used by delegated code and fix turns.
type Repository struct {
	Dir      string
	Watchdog shell.Watchdog
}

// ResolveCommit resolves a branch or commit expression to an immutable commit.
func (r Repository) ResolveCommit(ctx context.Context, ref string) (string, error) {
	return r.output(ctx, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
}

// ValidateBranch checks Git's branch-name rules.
func (r Repository) ValidateBranch(ctx context.Context, branch string) error {
	_, err := r.output(ctx, "check-ref-format", "--branch", branch)

	return err
}

// BranchExists reports whether a local branch already exists.
func (r Repository) BranchExists(ctx context.Context, branch string) bool {
	_, err := r.output(ctx, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)

	return err == nil
}

// AddWorktree creates a new branch and its checkout, or reattaches an existing branch.
func (r Repository) AddWorktree(ctx context.Context, path, branch, since string, existing bool) error {
	args := []string{"worktree", "add", "-b", branch, path, since}
	if existing {
		args = []string{"worktree", "add", path, branch}
	}

	_, err := r.output(ctx, args...)

	return err
}

// WorktreeOf finds the checkout where a branch is checked out.
func (r Repository) WorktreeOf(ctx context.Context, branch string) (string, error) {
	text, err := r.output(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return "", err
	}

	for _, block := range strings.Split(text, "\n\n") {
		lines := strings.Split(block, "\n")
		found := ""
		matches := false

		for _, line := range lines {
			if strings.HasPrefix(line, "worktree ") {
				found = strings.TrimPrefix(line, "worktree ")
			}

			if line == "branch refs/heads/"+branch {
				matches = true
			}
		}

		if matches {
			return found, nil
		}
	}

	return "", nil
}

// CompletionProblem proves a clean checkout, branch, ancestry and a new commit.
func (r Repository) CompletionProblem(ctx context.Context, branch, since string) string {
	status, err := r.output(ctx, "status", "--porcelain")
	if err != nil {
		return err.Error()
	}

	if status != "" {
		return "the worktree is not clean: " + status
	}

	checked, err := r.output(ctx, "symbolic-ref", "--quiet", "HEAD")
	if err != nil || checked != "refs/heads/"+branch {
		return "the recorded branch is no longer checked out"
	}

	_, err = r.output(ctx, "merge-base", "--is-ancestor", since, "refs/heads/"+branch)
	if err != nil {
		return "the starting commit is not a proven ancestor"
	}

	count, err := r.output(ctx, "rev-list", "--count", since+"..refs/heads/"+branch)
	if err != nil || count == "0" || count == "" {
		return "the branch has no proven new commit beyond " + since
	}

	return ""
}

func (r Repository) output(ctx context.Context, args ...string) (string, error) {
	watch := r.Watchdog
	if watch.Limit == 0 {
		watch.Limit = time.Minute
	}

	result, err := shell.Watch(ctx, append([]string{Bin}, args...), shell.Options{Dir: r.Dir}, watch)
	if err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), result.Err())
	}

	return strings.TrimSpace(result.Stdout), nil
}
