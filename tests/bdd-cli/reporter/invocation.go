package reporter

import (
	"strconv"
	"strings"
)

// Invocation is the command a subprocess was spawned with.
type Invocation struct {
	Binary string
	Args   []string
	Dir    string
	// Reconstructed marks a command the reporter derived from logged
	// options rather than read from a log record. Shown to the reader,
	// because a reconstructed command is a claim about what the engine
	// would build, not a record of what it did.
	Reconstructed bool
	Known         bool
}

// Command renders the invocation as a copyable shell line.
func (i Invocation) Command() string {
	if !i.Known {
		return ""
	}

	parts := make([]string, 0, len(i.Args)+1)
	parts = append(parts, i.Binary)

	for _, arg := range i.Args {
		parts = append(parts, quoteArg(arg))
	}

	return strings.Join(parts, " ")
}

// quoteArg quotes an argument only when the shell would need it.
func quoteArg(arg string) string {
	if arg == "" {
		return `''`
	}

	if !strings.ContainsAny(arg, " \t\n\"'\\$&|<>()") {
		return arg
	}

	return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
}

// Base flags src/claudecode/internal/cli/discovery.go always passes.
// Mirrored here only for the reconstruction path — logs written before
// the engine started recording its argv still get a command line, and
// it is labelled as reconstructed wherever it is shown.
func claudeBaseArgs() []string {
	return []string{
		"--output-format", "stream-json", "--verbose",
		"--input-format", "stream-json",
	}
}

// claudePermissionMode is what ClaudeProvider.buildClientOptions pins.
const claudePermissionMode = "acceptEdits"

// reconstructClaudeInvocation rebuilds the claude command from the
// options the engine did log: the tool allow/deny lists, the model, and
// the system prompt's size. Flag order follows BuildCommand's, so the
// result is comparable to a real one.
func reconstructClaudeInvocation(turn *Turn) Invocation {
	args := claudeBaseArgs()

	if len(turn.AllowedTools) > 0 {
		args = append(args, "--allowed-tools", strings.Join(turn.AllowedTools, ","))
	}

	if len(turn.DisallowedTools) > 0 {
		args = append(args, "--disallowed-tools", strings.Join(turn.DisallowedTools, ","))
	}

	if turn.SystemLength > 0 {
		args = append(args, "--system-prompt",
			"<"+strconv.Itoa(turn.SystemLength)+" bytes>")
	}

	if turn.Model != "" {
		args = append(args, "--model", turn.Model)
	}

	args = append(args, "--permission-mode", claudePermissionMode)

	return Invocation{
		Binary:        cliClaude,
		Args:          args,
		Reconstructed: true,
		Known:         true,
	}
}

// resolveInvocation prefers the logged command and falls back to
// reconstruction, which only the claude provider needs: crush and codex
// have logged their argv from the start.
func resolveInvocation(turn *Turn) Invocation {
	if turn.Invocation.Known {
		return turn.Invocation
	}

	if turn.CLI == cliClaude {
		return reconstructClaudeInvocation(turn)
	}

	return Invocation{}
}
