package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
)

// PostToolUseParams is the event, reduced to what a gate acts on. The payload
// carries more — session id, transcript path, the tool's response — and this
// package models a field when something needs it, not before.
type PostToolUseParams struct {
	// ToolName is the tool that just ran, e.g. "Write" or "Edit".
	ToolName string
	// FilePath is what that tool wrote, absolute. Empty for a tool that wrote
	// no file, which is most of them.
	FilePath string
}

// PostToolUseFunc answers one event. A nil error is silence and the tool call
// stands; a non-nil one blocks it, with the error's message as the reason
// Claude Code shows and acts on.
type PostToolUseFunc func(params PostToolUseParams, log *slog.Logger) error

// PostToolUse reads the event from in, hands it to answer, and writes the
// verdict to out. An unreadable payload is silence: a hook that cannot parse
// its input has found nothing, which is not the same as finding nothing wrong.
func PostToolUse(in io.Reader, out io.Writer, answer PostToolUseFunc) error {
	payload, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("reading the tool payload: %w", err)
	}

	var event struct {
		ToolName  string `json:"tool_name"`
		ToolInput struct {
			FilePath string `json:"file_path"`
		} `json:"tool_input"`
	}

	err = json.Unmarshal(payload, &event)
	if err != nil {
		return nil //nolint:nilerr // unreadable input is silence, not a verdict.
	}

	params := PostToolUseParams{ToolName: event.ToolName, FilePath: event.ToolInput.FilePath}

	found := answer(params, slog.Default().With("hook", "PostToolUse", "tool", params.ToolName))
	if found == nil {
		return nil
	}

	return block(out, found.Error())
}
