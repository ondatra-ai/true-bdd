package history

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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
// Claude Code, exported by the shim for /new-task, which isn't a hook) or
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

func (h *Hook) historyDir() string {
	return filepath.Join(h.repo, "docs", "history")
}

// stateFile holds a single line: the current task file's name, nothing else.
func (h *Hook) stateFile() string {
	return filepath.Join(h.historyDir(), "hook-state")
}

func (h *Hook) cursorDir() string {
	return filepath.Join(h.repo, "tmp", "history-cursor")
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

	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()

	full := args
	if dir != "" {
		full = append([]string{"-C", dir}, args...)
	}

	var stdout bytes.Buffer

	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		return "", err //nolint:wrapcheck // the caller only asks whether it worked.
	}

	return strings.TrimSpace(stdout.String()), nil
}
