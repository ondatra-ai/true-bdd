package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ondatra-ai/true-bdd/pkg/enginelog"
	"log/slog"
	"strings"

	claudecode "github.com/ondatra-ai/true-bdd/services/bdd-cli/claudecode"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

// ClaudeProvider runs a turn through the `claude` CLI via the in-process SDK
// wrapper. It is the only provider with native system-prompt and per-tool
// allowlist support, so ExecutionMode permissions apply directly, unprojected.
type ClaudeProvider struct {
	// No persistent client needed with severity1 SDK
}

// NewClaudeProvider creates the Claude provider. Stateless — the SDK
// builds a client per request.
func NewClaudeProvider() *ClaudeProvider {
	return &ClaudeProvider{}
}

// Name returns the CLI's config name.
func (c *ClaudeProvider) Name() string {
	return "claude"
}

// claudeAnswer holds a turn's two channels. Under a result schema the
// answer rides the result message; the text blocks carry only whatever
// narration preceded it.
type claudeAnswer struct {
	text    strings.Builder
	payload string
}

// Execute runs one single-turn prompt. The router owns the timeout.
func (c *ClaudeProvider) Execute(ctx context.Context, req Request) (string, error) {
	c.logPromptExecution(req.SystemPrompt, req.UserPrompt)

	opts := c.buildClientOptions(req.SystemPrompt, req.Model, req.Mode, req.ResultSchema)

	var answer claudeAnswer

	err := claudecode.WithClient(ctx, func(client claudecode.Client) error {
		return c.executeQuery(ctx, client, req.UserPrompt, &answer)
	}, opts...)

	if req.ResultSchema == "" {
		return c.handleExecutionResult(answer.text.String(), err)
	}

	// A schema-constrained turn that came back without its object is an
	// infrastructure failure; falling back to the narration would grade
	// whatever the model happened to say on the way to the answer.
	if err == nil && answer.payload == "" {
		return "", pkgerrors.ErrClaudeStreamNoOutput
	}

	return c.handleExecutionResult(answer.payload, err)
}

func (c *ClaudeProvider) logPromptExecution(systemPrompt, userPrompt string) {
	if systemPrompt != "" {
		slog.Debug("Calling Claude with system prompt", "system_length", len(systemPrompt), "user_length", len(userPrompt))
	} else {
		slog.Debug("Calling Claude", "prompt_length", len(userPrompt))
	}
}

func (c *ClaudeProvider) buildClientOptions(
	systemPrompt, model string, mode ExecutionMode, resultSchema string,
) []claudecode.Option {
	var opts []claudecode.Option

	if systemPrompt != "" {
		opts = append(opts, claudecode.WithSystemPrompt(systemPrompt))
	}

	// No empty-model fallback: the registry validated the ref at
	// startup, so an empty model here is a bug worth surfacing rather
	// than silently downgrading the turn to some default model.
	opts = append(opts, claudecode.WithModel(model))
	opts = append(opts, claudecode.WithPermissionMode(claudecode.PermissionModeAcceptEdits))
	opts = append(opts, c.getToolOptions(mode)...)

	if resultSchema != "" {
		opts = append(opts, claudecode.WithJSONSchema(resultSchema))
	}

	return opts
}

func (c *ClaudeProvider) getToolOptions(mode ExecutionMode) []claudecode.Option {
	var opts []claudecode.Option

	if len(mode.AllowedTools) > 0 {
		slog.Debug("Claude tools configured", "allowed_tools", mode.AllowedTools)
		opts = append(opts, claudecode.WithAllowedTools(mode.AllowedTools...))
	}

	if len(mode.DisallowedTools) > 0 {
		slog.Debug("Claude tools configured", "disallowed_tools", mode.DisallowedTools)
		opts = append(opts, claudecode.WithDisallowedTools(mode.DisallowedTools...))
	}

	return opts
}

func (c *ClaudeProvider) executeQuery(
	ctx context.Context,
	client claudecode.Client,
	userPrompt string,
	answer *claudeAnswer,
) error {
	slog.Debug("Connected to Claude client")
	slog.Debug("Sending user prompt to Claude", "length", len(userPrompt))

	err := client.Query(ctx, userPrompt)
	if err != nil {
		slog.Error("Query failed", "error", err)

		return fmt.Errorf("failed to send query: %w", pkgerrors.ErrSendQueryFailed(err))
	}

	slog.Debug("Query sent successfully")

	return c.streamMessages(ctx, client, answer)
}

func (c *ClaudeProvider) streamMessages(
	ctx context.Context, client claudecode.Client, answer *claudeAnswer,
) error {
	slog.Debug("Starting message stream")

	iter := client.ReceiveResponse(ctx)
	if iter == nil {
		return pkgerrors.ErrClaudeStreamClosed
	}

	defer func() { _ = iter.Close() }()

	messageCount := 0

	for {
		message, err := iter.Next(ctx)
		if err != nil {
			if errors.Is(err, claudecode.ErrNoMoreMessages) {
				if messageCount == 0 {
					return pkgerrors.ErrClaudeStreamNoOutput
				}
				// Stream ended without ResultMessage but we got some messages
				return nil
			}

			return fmt.Errorf("stream error: %w", err)
		}

		messageCount++
		slog.Debug("Message received", "count", messageCount, "type", fmt.Sprintf("%T", message))

		done, processErr := c.processMessage(message, answer)
		if processErr != nil {
			return processErr
		}

		if done {
			return nil
		}
	}
}

func (c *ClaudeProvider) processMessage(message any, answer *claudeAnswer) (bool, error) {
	switch msg := message.(type) {
	case *claudecode.AssistantMessage:
		c.processAssistantMessage(msg, &answer.text)
	case *claudecode.UserMessage:
		slog.Debug("UserMessage received")
		slog.Debug("UserMessage content", "content", fmt.Sprintf("%+v", msg))
	case *claudecode.SystemMessage:
		slog.Debug("SystemMessage received")
		slog.Debug("SystemMessage content", "content", fmt.Sprintf("%+v", msg))
	case *claudecode.ResultMessage:
		return c.processResultMessage(msg, answer)
	default:
		slog.Debug("Unhandled message type", "type", fmt.Sprintf("%T", message))
		slog.Debug("Unhandled message content", "content", fmt.Sprintf("%+v", message))
	}

	return false, nil
}

func (c *ClaudeProvider) processAssistantMessage(msg *claudecode.AssistantMessage, result *strings.Builder) {
	slog.Debug(enginelog.MsgAssistant, "content_blocks", len(msg.Content))
	slog.Debug("AssistantMessage content", "content", fmt.Sprintf("%+v", msg))

	for i, block := range msg.Content {
		slog.Debug("Processing content block", "index", i, "type", fmt.Sprintf("%T", block))
		c.processContentBlock(block, result)
	}
}

func (c *ClaudeProvider) processContentBlock(block any, result *strings.Builder) {
	if textBlock, ok := block.(*claudecode.TextBlock); ok {
		slog.Debug("TextBlock received")
		slog.Debug("TextBlock content", "text", textBlock.Text)
		result.WriteString(textBlock.Text)
	} else if toolUseBlock, ok := block.(*claudecode.ToolUseBlock); ok {
		c.logToolUseBlock(toolUseBlock)
	} else {
		c.logUnknownBlock(block)
	}
}

func (c *ClaudeProvider) logToolUseBlock(toolUseBlock *claudecode.ToolUseBlock) {
	slog.Debug("ToolUseBlock received")

	toolBytes, err := json.MarshalIndent(toolUseBlock, "      ", "  ")
	if err == nil {
		slog.Debug("ToolUseBlock details", "content", string(toolBytes))
	} else {
		slog.Debug("ToolUseBlock details (raw)", "content", fmt.Sprintf("%+v", toolUseBlock))
	}
}

func (c *ClaudeProvider) logUnknownBlock(block any) {
	slog.Debug("Unknown block type", "type", fmt.Sprintf("%T", block))

	blockBytes, err := json.MarshalIndent(block, "      ", "  ")
	if err == nil {
		slog.Debug("Unknown block content", "content", string(blockBytes))
	} else {
		slog.Debug("Unknown block content (raw)", "content", fmt.Sprintf("%+v", block))
	}
}

func (c *ClaudeProvider) processResultMessage(msg *claudecode.ResultMessage, answer *claudeAnswer) (bool, error) {
	slog.Debug(enginelog.MsgResult, "is_error", msg.IsError, "result", msg.Result)

	logTurnUsage(msg)

	if msg.IsError {
		return false, fmt.Errorf("claude returned error: %w", pkgerrors.ErrClaudeError(fmt.Sprintf("%v", msg.Result)))
	}

	answer.payload = resultPayload(msg)

	return true, nil
}

// resultPayload returns a schema-constrained turn's answer object,
// preferring the parsed form over the string the CLI also emits.
func resultPayload(msg *claudecode.ResultMessage) string {
	if msg.StructuredOutput != nil {
		encoded, err := json.Marshal(msg.StructuredOutput)
		if err == nil {
			return string(encoded)
		}

		slog.Warn("structured output not re-encodable", "error", err)
	}

	return strings.TrimSpace(msg.ResultText)
}

// logTurnUsage logs the cost/token counters as a standalone "AI turn usage"
// record: only the claude provider returns a usage report at all — crush
// and codex return bare text, so this can't merge into the router's line.
func logTurnUsage(msg *claudecode.ResultMessage) {
	// Cache reads and cache writes are priced differently from ordinary
	// input, so each counter is kept separate rather than summed.
	tokenKeys := []string{
		"input_tokens",
		"output_tokens",
		"cache_read_input_tokens",
		"cache_creation_input_tokens",
	}

	fields := []any{"cli", "claude"}

	if msg.TotalCostUSD != nil {
		fields = append(fields, "cost_usd", *msg.TotalCostUSD)
	}

	if msg.Usage != nil {
		for _, key := range tokenKeys {
			if count, ok := (*msg.Usage)[key]; ok {
				fields = append(fields, key, count)
			}
		}
	}

	// Nothing beyond the cli tag means the CLI reported no usage block;
	// a record saying so beats a silently missing one in a cost report.
	slog.Info(enginelog.MsgUsage, fields...)
}

func (c *ClaudeProvider) handleExecutionResult(resultStr string, err error) (string, error) {
	if err != nil {
		slog.Error("WithClient error", "error", err)

		errStr := err.Error()
		if strings.Contains(errStr, "token too long") || strings.Contains(errStr, "bufio.Scanner") {
			return resultStr, fmt.Errorf("response too large: %w", pkgerrors.ErrResponseTooLargeForBuffer(err))
		}

		return resultStr, fmt.Errorf("claude execution failed: %w", pkgerrors.ErrClaudeExecutionFailed(err))
	}

	slog.Info("Claude returned result", "length", len(resultStr))

	return resultStr, nil
}
