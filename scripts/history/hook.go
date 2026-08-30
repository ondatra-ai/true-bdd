package history

import (
	"os"
	"path/filepath"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
)

// DefaultRole is the writer when CLAUDE_HISTORY_ROLE says nothing: the main
// interactive session.
const DefaultRole = "claude"

// OffRole switches logging off entirely.
const OffRole = "0"

// Hook is one invocation's context: which repository is being logged, and who
// is writing. Both are fixed for the life of the process.
type Hook struct {
	repo string
	role string
}

// New builds a Hook for a repository root and a writer role.
func New(repo, role string) *Hook {
	if role == "" {
		role = DefaultRole
	}

	return &Hook{repo: repo, role: role}
}

// Role is the CLAUDE_HISTORY_ROLE of this process, defaulted.
func Role() string {
	if role := os.Getenv("CLAUDE_HISTORY_ROLE"); role != "" {
		return role
	}

	return DefaultRole
}

// RepoRoot locates the repository being logged: CLAUDE_PROJECT_DIR (set by
// Claude Code, exported by the shim for /task-start, which isn't a hook) or
// `git rev-parse --show-toplevel` — a `go run` binary's own path is a temp dir.
func RepoRoot() string {
	if dir := os.Getenv("CLAUDE_PROJECT_DIR"); dir != "" {
		resolved, err := filepath.Abs(dir)
		if err == nil {
			return resolved
		}
	}

	top, err := gitOutput("", "rev-parse", "--show-toplevel")
	if err == nil && top != "" {
		return top
	}

	working, err := os.Getwd()
	if err != nil {
		return "."
	}

	return working
}

// gitSHA stamps each entry with the commit it was written against, or "-"
// when git cannot say. Never an error: a missing stamp is not a reason to
// drop a turn.
func (h *Hook) gitSHA() string {
	sha, err := gitOutput(h.repo, "rev-parse", "--short", "HEAD")
	if err != nil || sha == "" {
		return "-"
	}

	return sha
}

// gitOutput runs one short git query. The two-second bound is the Python's:
// long enough for a cold index, short enough that a wedged git never eats the
// hook's timeout.
func gitOutput(dir string, args ...string) (string, error) {
	const budget = 2 * time.Second

	full := args
	if dir != "" {
		full = append([]string{"-C", dir}, args...)
	}

	out, err := git.OutputWith(cli.Options{Timeout: budget}, full...)
	if err != nil {
		return "", err
	}

	return out, nil
}
