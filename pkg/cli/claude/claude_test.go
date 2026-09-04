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
		"--setting-sources", "project",
		"--strict-mcp-config",
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

// The zero value is a prompt and the isolation preamble — never a bare -p.
// A turn that could opt back into the operator's user settings is how this
// package acquired an advisor nobody configured, so isolation is not a knob.
func TestArgsZeroValueIsIsolatedPrompt(t *testing.T) {
	t.Parallel()

	want := []string{"--setting-sources", "project", "--strict-mcp-config", "-p", "body"}
	if got := (claude.Options{}).Args("body"); !slices.Equal(got, want) {
		t.Errorf("args:\n got %q\nwant %q", got, want)
	}
}

// A schema-less turn still has to report what it cost, so RunJSON's envelope
// is reachable without one.
func TestArgsEnvelopeWithoutSchema(t *testing.T) {
	t.Parallel()

	got := claude.Options{Envelope: true}.Args("body")
	if !slices.Contains(got, "--output-format") || slices.Contains(got, "--json-schema") {
		t.Errorf("envelope without schema: got %q", got)
	}
}

// The advisor is a server-side tool: --disallowed-tools cannot reach it, and
// this variable is the only thing that can. Asserted through a real spawn.
func TestEnvDisablesTheAdvisor(t *testing.T) {
	// No t.Parallel: t.Setenv forbids it.
	t.Setenv("CLAUDE_CODE_DISABLE_ADVISOR_TOOL", "")

	result, err := shell.Run(t.Context(), []string{"/bin/sh", "-c", `echo "$CLAUDE_CODE_DISABLE_ADVISOR_TOOL"`},
		shell.Options{Env: claude.Options{}.Env()})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}

	if strings.TrimSpace(result.Stdout) != "1" {
		t.Errorf("advisor gate: got %q, want 1", strings.TrimSpace(result.Stdout))
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
