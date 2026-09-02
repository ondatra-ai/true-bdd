package codex_test

import (
	"slices"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/cli/codex"
)

// answerPath is the -o file every turn in these tests writes to.
const answerPath = "/tmp/run/answer.md"

func TestArgsCarryTheFlagsHeadlessCodexCannotRunWithout(t *testing.T) {
	t.Parallel()

	args := codex.Turn{
		Sandbox:    codex.SandboxWorkspaceWrite,
		WorkDir:    "/repo",
		Model:      "gpt-5.6-sol",
		AnswerPath: answerPath,
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
	if !slices.Equal(tail, []string{"-o", answerPath, "-"}) {
		t.Errorf("args tail = %v, want [-o <path> -]", tail)
	}
}

// codex takes a schema FILE, unlike claude's inline --json-schema. An
// unset SchemaPath must leave argv byte-identical: the flag rides the
// recorded argv, and argv is what a cassette's request hash covers.
func TestArgsCarryTheOutputSchemaOnlyWhenAsked(t *testing.T) {
	t.Parallel()

	base := codex.Turn{
		Sandbox:    codex.SandboxWorkspaceWrite,
		WorkDir:    "/repo",
		Model:      "gpt-5.6-sol",
		AnswerPath: answerPath,
	}

	if slices.Contains(base.Args(), "--output-schema") {
		t.Errorf("args = %v, want no schema flag when SchemaPath is unset", base.Args())
	}

	base.SchemaPath = "/tmp/run/codex-schema.json"

	args := base.Args()
	if !slices.Contains(args, "--output-schema") || !slices.Contains(args, base.SchemaPath) {
		t.Errorf("args = %v, want --output-schema and its path", args)
	}
}
