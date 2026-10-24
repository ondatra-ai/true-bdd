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
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/cli/codex"
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

	if req.CodexWriter {
		var err error

		env, err = prepareCodexWriter(ctx, req)
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
		Writer:     req.CodexWriter,
	}

	result, runErr := turn.Run()
	trace := result.Transcript

	saveTranscript(artifactPath(req, "codex.log"), trace)

	if req.CodexWriter {
		saveTranscript(artifactPath(req, "codex-events.jsonl"), result.Events)
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
		if req.CodexWriter {
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

// codexSandbox projects an ExecutionMode onto codex's sandbox levels: there
// is no level between read-only and workspace-write (the whole root), so a
// scoped mode runs read-only — verified the `-o` answer file still lands there.
func codexSandbox(mode ExecutionMode) string {
	if mode.GrantsSourceWrites() {
		return codex.SandboxWorkspaceWrite
	}

	return codex.SandboxReadOnly
}
