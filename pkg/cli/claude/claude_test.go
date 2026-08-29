package claude_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/claude"
	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// The argv scripts/internal/claudecli built before this package existed. Its
// 14 callers depend on the order, so it is pinned rather than reviewed.
func TestArgsOrder(t *testing.T) {
	t.Parallel()

	opts := claude.Options{
		AllowedTools:   "Read,Glob",
		PermissionMode: "plan",
		Schema:         "/tmp/schema.json",
		Role:           "merge-triage",
		Timeout:        time.Minute,
	}

	want := []string{
		"--allowedTools", "Read,Glob",
		"--permission-mode", "plan",
		"--output-format", "json", "--json-schema", "/tmp/schema.json",
		"-p", "the prompt",
	}

	got := opts.Args("the prompt")
	if !slices.Equal(got, want) {
		t.Errorf("args:\n got %q\nwant %q", got, want)
	}
}

// -p is last, always: anything after it would be read as the prompt's value.
func TestArgsPromptIsLast(t *testing.T) {
	t.Parallel()

	args := claude.Options{AllowedTools: "Read"}.Args("body")
	if args[len(args)-2] != "-p" || args[len(args)-1] != "body" {
		t.Errorf("args must end with -p and the prompt: %q", args)
	}
}

func TestArgsZeroValueIsBarePrompt(t *testing.T) {
	t.Parallel()

	got := claude.Options{}.Args("body")
	if !slices.Equal(got, []string{"-p", "body"}) {
		t.Errorf("args: got %q, want [-p body]", got)
	}
}

// CLAUDECODE is REMOVED, not blanked: a nested `claude -p` must look entirely
// unlaunched-from-a-session. Asserted through a real spawn, because a blanked
// key and a removed one are indistinguishable to anything that reads a value.
func TestEnvRemovesClaudecodeAndStampsTheRole(t *testing.T) {
	// No t.Parallel: t.Setenv forbids it.
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_HISTORY_ROLE", "stale")

	script := `if [ -n "${CLAUDECODE+x}" ]; then echo present; else echo absent; fi
echo "$CLAUDE_HISTORY_ROLE"`

	result, err := shell.Run(t.Context(), []string{"/bin/sh", "-c", script},
		shell.Options{Env: claude.Options{Role: "merge-fix"}.Env()})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("probe printed %q", result.Stdout)
	}

	if lines[0] != "absent" {
		t.Error("CLAUDECODE is still present; it must be removed, not blanked")
	}

	if lines[1] != "merge-fix" {
		t.Errorf("role: got %q, want %q — the stale value must not survive", lines[1], "merge-fix")
	}
}
