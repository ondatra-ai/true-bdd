package reporter

import (
	"strings"
	"testing"
)

// TestReconstructedClaudeInvocation pins the reconstruction against the flag
// order in src/claudecode/internal/cli/discovery.go's BuildCommand — nothing
// else catches the two drifting apart.
func TestReconstructedClaudeInvocation(t *testing.T) {
	turn := &Turn{
		CLI:             cliClaude,
		Model:           "claude-opus-4-8",
		SystemLength:    2154,
		AllowedTools:    []string{"Read(**)", "Write(./tmp/**)"},
		DisallowedTools: []string{"Bash", "Edit(**)"},
	}

	invocation := resolveInvocation(turn)

	if !invocation.Reconstructed {
		t.Error("a turn with no logged argv should be flagged as reconstructed")
	}

	command := invocation.Command()

	wantOrder := []string{
		"claude",
		"--output-format stream-json",
		"--verbose",
		"--input-format stream-json",
		"--allowed-tools",
		"--disallowed-tools",
		"--system-prompt",
		"--model claude-opus-4-8",
		"--permission-mode acceptEdits",
	}

	position := 0

	for _, fragment := range wantOrder {
		index := strings.Index(command[position:], fragment)
		if index < 0 {
			t.Fatalf("missing or out of order: %q in\n%s", fragment, command)
		}

		position += index
	}

	// The prompt itself is never inlined — only its size.
	if !strings.Contains(command, "<2154 bytes>") {
		t.Errorf("system prompt should appear as a size, got:\n%s", command)
	}
}

// TestLoggedInvocationWins checks that a real record beats the
// reconstruction, which is the whole point of logging the argv.
func TestLoggedInvocationWins(t *testing.T) {
	turn := &Turn{
		CLI:   cliCrush,
		Model: "zhipu-coding/glm-5.2",
		Invocation: Invocation{
			Binary: "crush",
			Args:   []string{"run", "--quiet", "-m", "zhipu-coding/glm-5.2"},
			Dir:    "/tmp/run",
			Known:  true,
		},
	}

	invocation := resolveInvocation(turn)

	if invocation.Reconstructed {
		t.Error("a logged invocation must not be flagged as reconstructed")
	}

	if got, want := invocation.Command(), "crush run --quiet -m zhipu-coding/glm-5.2"; got != want {
		t.Errorf("command = %q, want %q", got, want)
	}
}

// TestNonClaudeWithoutRecordIsUnknown checks the reporter does not
// invent a command for a CLI whose argv it cannot know.
func TestNonClaudeWithoutRecordIsUnknown(t *testing.T) {
	invocation := resolveInvocation(&Turn{CLI: cliCodex})
	if invocation.Known {
		t.Errorf("expected no command for an unlogged codex turn, got %q",
			invocation.Command())
	}
}

// TestQuoteArg checks arguments survive a round trip through a shell
// without changing meaning.
func TestQuoteArg(t *testing.T) {
	cases := map[string]string{
		testPlain:       testPlain,
		"--model":       "--model",
		"a b":           "'a b'",
		"Read(**)":      "'Read(**)'",
		"it's":          `'it'\''s'`,
		"":              `''`,
		"Bash,Edit(**)": "'Bash,Edit(**)'",
	}

	for in, want := range cases {
		if got := quoteArg(in); got != want {
			t.Errorf("quoteArg(%q) = %q, want %q", in, got, want)
		}
	}
}
