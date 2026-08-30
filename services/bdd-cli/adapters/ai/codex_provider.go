package ai

import (
	"context"
	"os"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/codex"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
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

	turn := codex.Turn{
		Sandbox:    codexSandbox(req.Mode),
		WorkDir:    req.WorkDir,
		Model:      req.Model,
		AnswerPath: answerPath,
		Prompt:     composePrompt(req),
		Env:        os.Environ(),
	}

	trace, runErr := turn.Run()

	saveTranscript(artifactPath(req, "codex.log"), trace)

	if runErr != nil {
		return trace, pkgerrors.ErrProviderExecutionFailed(p.Name(), runErr)
	}

	answer, readErr := disk.Read(answerPath)
	if readErr != nil {
		return trace, pkgerrors.ErrProviderExecutionFailed(p.Name(), readErr)
	}

	if strings.TrimSpace(string(answer)) == "" {
		return "", pkgerrors.ErrProviderProducedNoOutput(p.Name())
	}

	return string(answer), nil
}

// codexSandbox projects an ExecutionMode onto codex's sandbox levels: there
// is no level between read-only and workspace-write (the whole root), so a
// scoped mode runs read-only — verified the `-o` answer file still lands there.
func codexSandbox(mode ExecutionMode) string {
	if mode.GrantsSourceWrites() {
		return codex.SandboxWorkspaceWrite
	}

	return codex.SandboxReadOnly
}
