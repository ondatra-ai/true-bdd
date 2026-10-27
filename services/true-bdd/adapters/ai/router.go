package ai

import (
	"context"
	"fmt"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/enginelog"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

// Router dispatches each turn to the provider named by the ModelRef's CLI.
// It is the sole implementation of the domain's AIPort, so changing which
// CLI runs a checklist role is a config edit, not a code change.
type Router struct {
	providers map[provider.CLI]Provider
	workDir   string
	tmpDir    string
	// turns numbers each dispatched turn so subprocess providers can
	// archive transcripts without overwriting each other.
	turns atomic.Uint64
}

// NewRouter registers every supported provider. workDir is the host
// project root the turns run in; tmpDir is the run's artifact
// directory, where providers drop generated configs and transcripts.
func NewRouter(workDir, tmpDir string) *Router {
	providers := map[provider.CLI]Provider{
		provider.CLIClaude: NewClaudeProvider(),
		provider.CLICodex:  NewCodexProvider(),
	}

	return &Router{providers: providers, workDir: workDir, tmpDir: tmpDir}
}

// ExecutePromptWithSystem satisfies ports.AIPort: ModelRef picks the binary
// and model; Role says which checklist turn this is, so emitted telemetry
// is attributable to a stage of the walk.
func (r *Router) ExecutePromptWithSystem(
	ctx context.Context,
	role provider.Role,
	systemPrompt string,
	userPrompt string,
	model provider.ModelRef,
	mode ExecutionMode,
	resultSchema string,
	options ...provider.TurnOptions,
) (string, error) {
	target, ok := r.providers[model.CLI]
	if !ok {
		return "", pkgerrors.ErrProviderNotRegisteredForCLI(string(model.CLI))
	}

	turn := r.turns.Add(1)
	label := fmt.Sprintf("%s-turn%03d", model.CLI, turn)

	slog.Info(enginelog.MsgDispatch,
		"turn", turn,
		"label", label,
		"role", string(role),
		"cli", string(model.CLI),
		"model", model.Model,
		"system_length", len(systemPrompt),
		"user_length", len(userPrompt),
	)

	turnOptions := writerOptions(role, model.CLI, options)

	started := time.Now()

	response, err := target.Execute(ctx, Request{
		CodexApply:   role == provider.RoleApply && model.CLI == provider.CLICodex,
		ClaudeWriter: turnOptions.ClaudeWriter,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Model:        model.Model,
		Mode:         mode,
		WorkDir:      r.workDir,
		TmpDir:       r.tmpDir,
		Label:        label,
		Role:         string(role),
		ResultSchema: resultSchema,
		CodexWriter:  turnOptions.CodexWriter,
	})

	// Failed turns retain timing evidence alongside successful turns.
	elapsed := time.Since(started)

	if err != nil {
		slog.Info(enginelog.MsgFailed,
			"turn", turn, "label", label, "role", string(role),
			"cli", string(model.CLI), "model", model.Model,
			"duration_ms", elapsed.Milliseconds(), "error", err.Error(),
		)

		// Providers already return pkgerrors-wrapped failures naming
		// the cli; wrapping again here would only add noise.
		//nolint:wrapcheck // already wrapped at the provider boundary
		return response, err
	}

	slog.Info(enginelog.MsgReturned,
		"turn", turn,
		"label", label,
		"role", string(role),
		"cli", string(model.CLI),
		"model", model.Model,
		"duration_ms", elapsed.Milliseconds(),
		"length", len(response),
	)

	return response, nil
}

func writerOptions(role provider.Role, cli provider.CLI, options []provider.TurnOptions) provider.TurnOptions {
	if role != provider.RoleApply || len(options) == 0 {
		return provider.TurnOptions{}
	}

	return provider.TurnOptions{
		ClaudeWriter: cli == provider.CLIClaude && options[0].ClaudeWriter,
		CodexWriter:  cli == provider.CLICodex && options[0].CodexWriter,
	}
}
