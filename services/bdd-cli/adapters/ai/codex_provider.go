package ai

import (
	"context"
	"os"
	"strings"

	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

const (
	// codexSandboxReadOnly / codexSandboxWorkspaceWrite are codex's two
	// usable sandbox levels. One of them is MANDATORY: without `-s`,
	// `codex exec` blocks on an approval prompt and hangs forever
	// headlessly.
	codexSandboxReadOnly       = "read-only"
	codexSandboxWorkspaceWrite = "workspace-write"
)

// CodexProvider runs a turn through the `codex` CLI.
//
// codex prints only a trace to stdout, so the assistant's answer is
// read from the file named by `-o`. Its sandbox is coarser than
// Claude's per-tool allowlist and crush's guard hook: workspace-write
// grants the whole working root rather than the mode's specific write
// globs, and there is no narrower level. So only a mode that
// deliberately opens a project tree gets it (see codexSandbox);
// everything else runs read-only, and the Go side recovers
// FILE_START/FILE_END content from the response rather than from a file
// the model wrote.
type CodexProvider struct{}

// NewCodexProvider creates the codex provider.
func NewCodexProvider() *CodexProvider {
	return &CodexProvider{}
}

// Name returns the CLI's config name.
func (p *CodexProvider) Name() string {
	return "codex"
}

// Execute runs one turn and returns the final message codex wrote to
// its output file.
func (p *CodexProvider) Execute(ctx context.Context, req Request) (string, error) {
	answerPath := artifactPath(req, "codex-answer.md")
	if answerPath == "" {
		return "", pkgerrors.ErrProviderExecutionFailed(p.Name(), pkgerrors.ErrCreateTmpDirectory)
	}

	invocation := cliInvocation{
		Binary:         "codex",
		Args:           buildCodexArgs(req, answerPath),
		Dir:            req.WorkDir,
		Env:            os.Environ(),
		Stdin:          composePrompt(req),
		TranscriptPath: artifactPath(req, "codex.log"),
	}

	trace, runErr := invocation.run(ctx)
	if runErr != nil {
		return trace, pkgerrors.ErrProviderExecutionFailed(p.Name(), runErr)
	}

	answer, readErr := os.ReadFile(answerPath)
	if readErr != nil {
		return trace, pkgerrors.ErrProviderExecutionFailed(p.Name(), readErr)
	}

	if strings.TrimSpace(string(answer)) == "" {
		return "", pkgerrors.ErrProviderProducedNoOutput(p.Name())
	}

	return string(answer), nil
}

// buildCodexArgs builds the argv for one headless turn. Kept pure so
// it is unit-testable without spawning codex.
func buildCodexArgs(req Request, answerPath string) []string {
	args := []string{
		"exec",
		"-s", codexSandbox(req.Mode),
		// No session files: every engine turn is single-shot.
		"--ephemeral",
		// BDD fixture tmpdirs are not git repos, and codex refuses to
		// run outside one unless told not to care.
		"--skip-git-repo-check",
		"--color", "never",
	}

	if req.WorkDir != "" {
		args = append(args, "-C", req.WorkDir)
	}

	if req.Model != "" {
		args = append(args, "-m", req.Model)
	}

	// `-o` captures the final message; stdout carries only the trace.
	// The trailing `-` makes codex read the prompt from stdin.
	return append(args, "-o", answerPath, "-")
}

// codexSandbox projects an ExecutionMode onto codex's sandbox levels.
//
// Only a mode that deliberately opens a project tree gets
// workspace-write. A mode whose sole write grant is the tmp glob runs
// READ-ONLY, because codex has no level between "nothing" and "all of
// the working root": handing workspace-write to a validation or
// fix-generation turn would let it edit the very tree it is only
// supposed to read.
//
// Scratch writes are not lost by this. codex's answer reaches the
// engine through the `-o` file, which the CLI writes itself rather than
// the sandboxed model — verified to still be produced under
// `-s read-only` — and file content the model proposes is recovered
// from FILE_START/FILE_END markers in that answer.
//
// `--add-dir` cannot narrow this: it only widens workspace-write with
// extra roots, so there is no way to express "the tmp dir only".
func codexSandbox(mode ExecutionMode) string {
	if mode.GrantsSourceWrites() {
		return codexSandboxWorkspaceWrite
	}

	return codexSandboxReadOnly
}
