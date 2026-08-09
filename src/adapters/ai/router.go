package ai

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/ondatra-ai/true-bdd/src/internal/domain/models/provider"
	pkgerrors "github.com/ondatra-ai/true-bdd/src/internal/pkg/errors"
)

// aiPromptTimeout bounds one turn regardless of which CLI runs it.
const aiPromptTimeout = 20 * time.Minute

// Router dispatches each turn to the provider named by the ModelRef's
// CLI. It is the sole implementation of the domain's AIPort, so
// changing which CLI runs a checklist role is a config edit rather
// than a code change.
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
		provider.CLICrush:  NewCrushProvider(),
		provider.CLICodex:  NewCodexProvider(),
	}

	return &Router{providers: providers, workDir: workDir, tmpDir: tmpDir}
}

// ExecutePromptWithSystem satisfies ports.AIPort. The ModelRef decides
// both which binary runs and which model it runs.
func (r *Router) ExecutePromptWithSystem(
	ctx context.Context,
	systemPrompt string,
	userPrompt string,
	model provider.ModelRef,
	mode ExecutionMode,
) (string, error) {
	target, ok := r.providers[model.CLI]
	if !ok {
		return "", pkgerrors.ErrProviderNotRegisteredForCLI(string(model.CLI))
	}

	slog.Info("Dispatching AI turn",
		"cli", string(model.CLI),
		"model", model.Model,
		"system_length", len(systemPrompt),
		"user_length", len(userPrompt),
	)

	timeoutCtx, cancel := context.WithTimeout(ctx, aiPromptTimeout)
	defer cancel()

	response, err := target.Execute(timeoutCtx, Request{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Model:        model.Model,
		Mode:         mode,
		WorkDir:      r.workDir,
		TmpDir:       r.tmpDir,
		Label:        fmt.Sprintf("%s-turn%03d", model.CLI, r.turns.Add(1)),
	})
	if err != nil {
		// Providers already return pkgerrors-wrapped failures naming
		// the cli; wrapping again here would only add noise.
		//nolint:wrapcheck // already wrapped at the provider boundary
		return response, err
	}

	slog.Info("AI turn returned", "cli", string(model.CLI), "length", len(response))

	return response, nil
}
