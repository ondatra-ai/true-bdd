package codex_test

import (
	"slices"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/cli/codex"
)

func TestArgsCarryTheFlagsHeadlessCodexCannotRunWithout(t *testing.T) {
	t.Parallel()

	args := codex.Turn{
		Sandbox:    codex.SandboxWorkspaceWrite,
		WorkDir:    "/repo",
		Model:      "gpt-5.6-sol",
		AnswerPath: "/tmp/run/answer.md",
	}.Args()

	// `-s` is mandatory: without it `codex exec` blocks on an approval prompt
	// and hangs headlessly forever.
	if !slices.Contains(args, "-s") || !slices.Contains(args, codex.SandboxWorkspaceWrite) {
		t.Errorf("args = %v, want a workspace-write sandbox flag", args)
	}

	// Fixture tmpdirs are not git repos.
	if !slices.Contains(args, "--skip-git-repo-check") {
		t.Errorf("args = %v, want --skip-git-repo-check", args)
	}

	if !slices.Contains(args, "--ephemeral") {
		t.Errorf("args = %v, want --ephemeral", args)
	}

	// The answer is read from the -o file; stdout is only a trace. And the
	// trailing `-` is what makes codex read the prompt from stdin.
	tail := args[len(args)-3:]
	if !slices.Equal(tail, []string{"-o", "/tmp/run/answer.md", "-"}) {
		t.Errorf("args tail = %v, want [-o <path> -]", tail)
	}
}
