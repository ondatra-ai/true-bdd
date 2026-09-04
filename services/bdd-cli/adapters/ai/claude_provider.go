package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/claude"
	"github.com/ondatra-ai/true-bdd/pkg/enginelog"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

// ClaudeProvider runs a turn through the `claude` CLI via pkg/cli/claude. It
// is the only provider with native system-prompt and per-tool allowlist
// support, so ExecutionMode permissions apply directly, unprojected.
type ClaudeProvider struct{}

// NewClaudeProvider creates the Claude provider. Stateless — a turn is a
// spawn, and there is nothing to keep between two of them.
func NewClaudeProvider() *ClaudeProvider {
	return &ClaudeProvider{}
}

// Name returns the CLI's config name.
func (c *ClaudeProvider) Name() string {
	return "claude"
}

// Execute runs one single-turn prompt. The router owns the timeout.
func (c *ClaudeProvider) Execute(ctx context.Context, req Request) (string, error) {
	c.logPromptExecution(req.SystemPrompt, req.UserPrompt)

	answer, err := claude.RunStream(req.UserPrompt, c.options(ctx, req), logEvent)
	if err != nil {
		return c.handleExecutionResult("", err)
	}

	logTurnUsage(answer)

	payload := answerText(answer.Data)

	// A schema-constrained turn that came back without its object is an
	// infrastructure failure; falling back to the narration would grade
	// whatever the model happened to say on the way to the answer.
	if req.ResultSchema != "" && payload == "" {
		return "", pkgerrors.ErrClaudeStreamNoOutput
	}

	return c.handleExecutionResult(payload, nil)
}

func (c *ClaudeProvider) logPromptExecution(systemPrompt, userPrompt string) {
	if systemPrompt != "" {
		slog.Debug("Calling Claude with system prompt", "system_length", len(systemPrompt), "user_length", len(userPrompt))
	} else {
		slog.Debug("Calling Claude", "prompt_length", len(userPrompt))
	}
}

// options projects one Request onto the wrapper's argv. No empty-model
// fallback: the registry validated the ref at startup, so an empty model here
// is a bug worth surfacing rather than a silent downgrade to some default.
func (c *ClaudeProvider) options(ctx context.Context, req Request) claude.Options {
	opts := claude.Options{
		SystemPrompt:   req.SystemPrompt,
		Model:          req.Model,
		PermissionMode: permissionAcceptEdits,
		Schema:         req.ResultSchema,
		Dir:            req.WorkDir,
		Timeout:        remaining(ctx),
	}

	if len(req.Mode.AllowedTools) > 0 {
		slog.Debug("Claude tools configured", "allowed_tools", req.Mode.AllowedTools)

		opts.AllowedTools = strings.Join(req.Mode.AllowedTools, ",")
	}

	if len(req.Mode.DisallowedTools) > 0 {
		slog.Debug("Claude tools configured", "disallowed_tools", req.Mode.DisallowedTools)

		opts.DisallowedTools = req.Mode.DisallowedTools
	}

	return opts
}

// permissionAcceptEdits is the mode every engine turn runs under: the turn's
// writes are already fenced by the allowlist and the fixture's own settings,
// so a prompt would only stall a run nobody is watching.
const permissionAcceptEdits = "acceptEdits"

// remaining is the router's own budget, read back off the context. The
// wrapper bounds a turn by duration rather than by cancellation, so the
// deadline has to be handed over as one.
func remaining(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0
	}

	return time.Until(deadline)
}

// answerText reads the envelope's payload. A schema turn's answer IS the
// object and rides back as its own JSON; a prose turn's is a JSON string, and
// unwrapping it is what tells the two apart.
func answerText(data json.RawMessage) string {
	var text string
	if json.Unmarshal(data, &text) == nil {
		return strings.TrimSpace(text)
	}

	return string(data)
}

// logEvent narrates one streaming record. These messages are the reporter's
// folding keys (pkg/testkit/reporter/engine_log.go): drop one and a column of
// the report UI goes empty on every future run.
func logEvent(event claude.Event) {
	switch event.Kind {
	case claude.EventAssistant:
		slog.Debug(enginelog.MsgAssistant, "content_blocks", event.Blocks)
	case claude.EventToolUse:
		logToolUse(event)
	case claude.EventResult:
		slog.Debug(enginelog.MsgResult)
	}
}

// logToolUse writes the block in the {name, input} shape reporter.parseToolCall
// decodes. A block that will not marshal is still worth its name.
func logToolUse(event claude.Event) {
	payload, err := json.Marshal(struct {
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	}{Name: event.Name, Input: event.Input})
	if err != nil {
		slog.Debug(enginelog.MsgToolUse, "content", event.Name)

		return
	}

	slog.Debug(enginelog.MsgToolUse, "content", string(payload))
}

// logTurnUsage logs the cost/token counters as a standalone "AI turn usage"
// record: only the claude provider returns a usage report at all — crush
// and codex return bare text, so this can't merge into the router's line.
func logTurnUsage(answer claude.Answer) {
	// Cache reads and cache writes are priced differently from ordinary
	// input, so each counter is kept separate rather than summed.
	tokenKeys := []string{
		"input_tokens",
		"output_tokens",
		"cache_read_input_tokens",
		"cache_creation_input_tokens",
	}

	fields := []any{"cli", "claude", "cost_usd", answer.CostUSD}

	for _, key := range tokenKeys {
		if count, ok := answer.Tokens[key]; ok {
			fields = append(fields, key, count)
		}
	}

	slog.Info(enginelog.MsgUsage, fields...)
}

func (c *ClaudeProvider) handleExecutionResult(resultStr string, err error) (string, error) {
	if err != nil {
		slog.Error("Claude turn failed", "error", err)

		errStr := err.Error()
		if strings.Contains(errStr, "token too long") || strings.Contains(errStr, "bufio.Scanner") {
			return resultStr, fmt.Errorf("response too large: %w", pkgerrors.ErrResponseTooLargeForBuffer(err))
		}

		return resultStr, fmt.Errorf("claude execution failed: %w", pkgerrors.ErrClaudeExecutionFailed(err))
	}

	slog.Info("Claude returned result", "length", len(resultStr))

	return resultStr, nil
}
