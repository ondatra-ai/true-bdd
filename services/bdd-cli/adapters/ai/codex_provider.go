package ai

import (
	"context"
	"os"
	"strings"

	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

const (
	// codexSandboxReadOnly / codexSandboxWorkspaceWrite are codex's two usable
	// sandbox levels. `-s` is MANDATORY: without it, `codex exec` blocks on an
	// approval prompt and hangs forever headlessly.
	codexSandboxReadOnly       = "read-only"
	codexSandboxWorkspaceWrite = "workspace-write"
)

// CodexProvider runs a turn through the `codex` CLI. codex prints only a
// trace to stdout; the answer is read from the file named by `-o`. Sandbox
// handling is coarser than the other providers' — see codexSandbox.
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

// codexSandbox projects an ExecutionMode onto codex's sandbox levels: there
// is no level between read-only and workspace-write (the whole root), so a
// scoped mode runs read-only — verified the `-o` answer file still lands there.
func codexSandbox(mode ExecutionMode) string {
	if mode.GrantsSourceWrites() {
		return codexSandboxWorkspaceWrite
	}

	return codexSandboxReadOnly
}
