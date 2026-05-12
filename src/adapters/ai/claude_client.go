package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	claudecode "bdd-cli/src/claudecode"
	pkgerrors "bdd-cli/src/internal/pkg/errors"
)

const aiPromptTimeout = 20 * time.Minute // Timeout for AI prompt execution

type ClaudeClient struct {
	// No persistent client needed with severity1 SDK
}

func NewClaudeClient() (*ClaudeClient, error) {
	// No initialization needed with severity1 SDK - clients are created per-request
	return &ClaudeClient{}, nil
}

func (c *ClaudeClient) Name() string {
	return "Claude"
}

func (c *ClaudeClient) ExecutePromptWithSystem(
	ctx context.Context,
	systemPrompt string,
	userPrompt string,
	model string,
	mode ExecutionMode,
) (string, error) {
	c.logPromptExecution(systemPrompt, userPrompt)

	timeoutCtx, cancel := context.WithTimeout(ctx, aiPromptTimeout)
	defer cancel()

	opts := c.buildClientOptions(systemPrompt, model, mode)

	var resultStr string

	err := claudecode.WithClient(timeoutCtx, func(client claudecode.Client) error {
		var queryErr error

		resultStr, queryErr = c.executeQuery(timeoutCtx, client, userPrompt)

		return queryErr
	}, opts...)

	return c.handleExecutionResult(resultStr, err)
}

func (c *ClaudeClient) logPromptExecution(systemPrompt, userPrompt string) {
	if systemPrompt != "" {
		slog.Debug("Calling Claude with system prompt", "system_length", len(systemPrompt), "user_length", len(userPrompt))
	} else {
		slog.Debug("Calling Claude", "prompt_length", len(userPrompt))
	}
}

func (c *ClaudeClient) buildClientOptions(systemPrompt, model string, mode ExecutionMode) []claudecode.Option {
	var opts []claudecode.Option

	if systemPrompt != "" {
		opts = append(opts, claudecode.WithSystemPrompt(systemPrompt))
	}

	opts = append(opts, c.getModelOption(model))
	opts = append(opts, claudecode.WithPermissionMode(claudecode.PermissionModeAcceptEdits))
	opts = append(opts, c.getToolOptions(mode)...)

	return opts
}

func (c *ClaudeClient) getModelOption(model string) claudecode.Option {
	if model != "" {
		return claudecode.WithModel(model)
	}

	return claudecode.WithModel("sonnet")
}

func (c *ClaudeClient) getToolOptions(mode ExecutionMode) []claudecode.Option {
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

func (c *ClaudeClient) executeQuery(ctx context.Context, client claudecode.Client, userPrompt string) (string, error) {
	slog.Debug("Connected to Claude client")
	slog.Debug("Sending user prompt to Claude", "length", len(userPrompt))

	err := client.Query(ctx, userPrompt)
	if err != nil {
		slog.Error("Query failed", "error", err)

		return "", fmt.Errorf("failed to send query: %w", pkgerrors.ErrSendQueryFailed(err))
	}

	slog.Debug("Query sent successfully")

	return c.streamMessages(ctx, client)
}

func (c *ClaudeClient) streamMessages(ctx context.Context, client claudecode.Client) (string, error) {
	var result strings.Builder

	slog.Debug("Starting message stream")

	iter := client.ReceiveResponse(ctx)
	if iter == nil {
		return "", pkgerrors.ErrClaudeStreamClosed
	}

	defer func() { _ = iter.Close() }()

	messageCount := 0

	for {
		message, err := iter.Next(ctx)
		if err != nil {
			if errors.Is(err, claudecode.ErrNoMoreMessages) {
				if messageCount == 0 {
					return "", pkgerrors.ErrClaudeStreamNoOutput
				}
				// Stream ended without ResultMessage but we got some messages
				return result.String(), nil
			}

			return result.String(), fmt.Errorf("stream error: %w", err)
		}

		messageCount++
		slog.Debug("Message received", "count", messageCount, "type", fmt.Sprintf("%T", message))

		done, processErr := c.processMessage(message, &result)
		if processErr != nil {
			return result.String(), processErr
		}

		if done {
			return result.String(), nil
		}
	}
}

func (c *ClaudeClient) processMessage(message any, result *strings.Builder) (bool, error) {
	switch msg := message.(type) {
	case *claudecode.AssistantMessage:
		c.processAssistantMessage(msg, result)
	case *claudecode.UserMessage:
		slog.Debug("UserMessage received")
		slog.Debug("UserMessage content", "msg", fmt.Sprintf("%+v", msg))
	case *claudecode.SystemMessage:
		slog.Debug("SystemMessage received")
		slog.Debug("SystemMessage content", "msg", fmt.Sprintf("%+v", msg))
	case *claudecode.ResultMessage:
		return c.processResultMessage(msg)
	default:
		slog.Debug("Unhandled message type", "type", fmt.Sprintf("%T", message))
		slog.Debug("Unhandled message content", "msg", fmt.Sprintf("%+v", message))
	}

	return false, nil
}

func (c *ClaudeClient) processAssistantMessage(msg *claudecode.AssistantMessage, result *strings.Builder) {
	slog.Debug("AssistantMessage received", "content_blocks", len(msg.Content))
	slog.Debug("AssistantMessage content", "msg", fmt.Sprintf("%+v", msg))

	for i, block := range msg.Content {
		slog.Debug("Processing content block", "index", i, "type", fmt.Sprintf("%T", block))
		c.processContentBlock(block, result)
	}
}

func (c *ClaudeClient) processContentBlock(block any, result *strings.Builder) {
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

func (c *ClaudeClient) logToolUseBlock(toolUseBlock *claudecode.ToolUseBlock) {
	slog.Debug("ToolUseBlock received")

	toolBytes, err := json.MarshalIndent(toolUseBlock, "      ", "  ")
	if err == nil {
		slog.Debug("ToolUseBlock details", "content", string(toolBytes))
	} else {
		slog.Debug("ToolUseBlock details (raw)", "content", fmt.Sprintf("%+v", toolUseBlock))
	}
}

func (c *ClaudeClient) logUnknownBlock(block any) {
	slog.Debug("Unknown block type", "type", fmt.Sprintf("%T", block))

	blockBytes, err := json.MarshalIndent(block, "      ", "  ")
	if err == nil {
		slog.Debug("Unknown block content", "content", string(blockBytes))
	} else {
		slog.Debug("Unknown block content (raw)", "content", fmt.Sprintf("%+v", block))
	}
}

func (c *ClaudeClient) processResultMessage(msg *claudecode.ResultMessage) (bool, error) {
	slog.Debug("ResultMessage received", "is_error", msg.IsError, "result", msg.Result)

	if msg.IsError {
		return false, fmt.Errorf("claude returned error: %w", pkgerrors.ErrClaudeError(fmt.Sprintf("%v", msg.Result)))
	}

	return true, nil
}

func (c *ClaudeClient) handleExecutionResult(resultStr string, err error) (string, error) {
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
