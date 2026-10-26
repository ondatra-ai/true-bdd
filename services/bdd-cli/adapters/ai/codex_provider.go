package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	codexcli "github.com/ondatra-ai/true-bdd/pkg/cli/codex"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/cli/codex"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

// CodexProvider runs a turn through the `codex` CLI. codex prints only a
// trace to stdout; the answer is read from the file named by `-o`. Codex
// applies use a named profile; planning and validation stay read-only.
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
	if !req.CodexApply && !req.CodexWriter {
		return p.execute(ctx, req, "")
	}

	home, err := codexWriterHome(req)
	if err != nil {
		return "", pkgerrors.ErrProviderExecutionFailed(p.Name(), err)
	}

	answer, turnErr := p.execute(ctx, req, home)

	cleanupErr := disk.Remove(filepath.Join(home, "auth.json"))
	if cleanupErr != nil {
		cleanupErr = fmt.Errorf("remove codex authentication: %w", cleanupErr)
	}

	return answer, errors.Join(turnErr, cleanupErr)
}

func (p *CodexProvider) execute(ctx context.Context, req Request, home string) (string, error) {
	answerPath := artifactPath(req, "codex-answer.txt")
	if answerPath == "" {
		return "", pkgerrors.ErrProviderExecutionFailed(p.Name(), pkgerrors.ErrCreateTmpDirectory)
	}

	schemaPath, err := codexSchemaPath(req)
	if err != nil {
		return "", err
	}

	sandbox := codexSandbox(req.Mode)
	env := os.Environ()

	var guard *codexGlobGuard

	if req.CodexApply || req.CodexWriter {
		var err error

		env, err = prepareCodexWriter(ctx, req, home)
		if err != nil {
			return "", pkgerrors.ErrProviderExecutionFailed(p.Name(), err)
		}

		guard, err = newCodexGlobGuard(req)
		if err != nil {
			return "", pkgerrors.ErrProviderExecutionFailed(p.Name(), err)
		}

		sandbox = "writer"
	}

	slog.Info("Codex turn sandboxed", "label", req.Label, "sandbox", sandbox)

	turn := codex.Turn{
		Sandbox:    sandbox,
		WorkDir:    req.WorkDir,
		Model:      req.Model,
		AnswerPath: answerPath,
		Prompt:     composePrompt(req),
		Env:        env,
		SchemaPath: schemaPath,
		Writer:     req.CodexApply || req.CodexWriter,
	}

	result, runErr := turn.Run(ctx)

	return finishCodexTurn(req, answerPath, result, runErr, guard)
}

func finishCodexTurn(
	req Request, answerPath string, result codex.Result, runErr error, guard *codexGlobGuard,
) (string, error) {
	trace := result.Transcript
	codexcli.WarnDenials(result.Events, req.Label, req.Role)

	saveTranscript(artifactPath(req, "codex.log"), trace)

	if guard != nil {
		saveTranscript(artifactPath(req, "codex-events.jsonl"), result.Events)

		globErr := guard.enforce()
		if globErr != nil {
			return trace, pkgerrors.ErrProviderExecutionFailed("codex", globErr)
		}
	}

	return codexAnswer(req, answerPath, result, runErr)
}

func codexSchemaPath(req Request) (string, error) {
	if req.ResultSchema == "" {
		return "", nil
	}

	path := artifactPath(req, "codex-schema.json")

	strict, err := codexStrictSchema(req.ResultSchema)
	if err != nil {
		return "", pkgerrors.ErrProviderExecutionFailed("codex", err)
	}

	err = disk.Write(path, []byte(strict), disk.Shared)
	if err != nil {
		return "", pkgerrors.ErrProviderExecutionFailed("codex", err)
	}

	return path, nil
}

func codexAnswer(req Request, answerPath string, result codex.Result, runErr error) (string, error) {
	trace := result.Transcript
	if runErr != nil {
		if req.CodexWriter && result.Started {
			return trace, &pkgerrors.CodexTurnError{Outcome: pkgerrors.CodexTurnNonZeroExit,
				Cause: pkgerrors.ErrProviderExecutionFailed("codex", runErr)}
		}

		return trace, pkgerrors.ErrProviderExecutionFailed("codex", runErr)
	}

	answer, readErr := disk.Read(answerPath)
	if readErr != nil {
		if req.CodexWriter && errors.Is(readErr, fs.ErrNotExist) {
			return trace, &pkgerrors.CodexTurnError{Outcome: pkgerrors.CodexTurnEmptyOutput,
				Cause: pkgerrors.ErrProviderProducedNoOutput("codex")}
		}

		return trace, pkgerrors.ErrProviderExecutionFailed("codex", readErr)
	}

	if strings.TrimSpace(string(answer)) == "" {
		if req.CodexApply || req.CodexWriter {
			return "", &pkgerrors.CodexTurnError{Outcome: pkgerrors.CodexTurnEmptyOutput,
				Cause: pkgerrors.ErrProviderProducedNoOutput("codex")}
		}

		return "", pkgerrors.ErrProviderProducedNoOutput("codex")
	}

	return string(answer), nil
}

// codexStrictSchema lists every property of every object in its `required`:
// codex refuses a schema that leaves one out (HTTP 400 `invalid_json_schema`,
// verified on codex-cli 0.162.1), though claude takes the same schema as is.
func codexStrictSchema(schema string) (string, error) {
	var root any

	err := json.Unmarshal([]byte(schema), &root)
	if err != nil {
		return "", fmt.Errorf("decode result schema: %w", err)
	}

	requireEveryProperty(root)

	strict, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode result schema: %w", err)
	}

	return string(strict), nil
}

// requireEveryProperty walks a decoded schema, setting each object's
// `required` to all of its `properties`, sorted so the file is stable.
func requireEveryProperty(node any) {
	switch typed := node.(type) {
	case map[string]any:
		properties, isObject := typed["properties"].(map[string]any)
		if isObject {
			typed["required"] = slices.Sorted(maps.Keys(properties))
		}

		for _, child := range typed {
			requireEveryProperty(child)
		}
	case []any:
		for _, child := range typed {
			requireEveryProperty(child)
		}
	}
}

// codexSandbox keeps planning and validation read-only regardless of their tool specs.
func codexSandbox(_ ExecutionMode) string {
	return codex.SandboxReadOnly
}
