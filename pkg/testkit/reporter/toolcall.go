package reporter

import (
	"encoding/json"
	"sort"
	"strings"
)

// ToolCall is one tool the model invoked during a turn — the only record
// of what a turn actually *did*, distinguishing a turn that made no tool
// calls at all from one that searched and found nothing.
type ToolCall struct {
	Name  string
	Input string
}

// toolUsePayload is the shape claude_provider.go marshals into the
// "ToolUseBlock details" record's content field.
type toolUsePayload struct {
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// parseToolCall decodes one tool-use record's content. Returns false for
// the raw-fallback form the provider emits when the block will not
// marshal, which carries no structure worth showing.
func parseToolCall(content string) (ToolCall, bool) {
	var payload toolUsePayload

	err := json.Unmarshal([]byte(content), &payload)
	if err != nil || payload.Name == "" {
		return ToolCall{}, false
	}

	return ToolCall{Name: payload.Name, Input: compactJSON(payload.Input)}, true
}

// compactJSON renders a tool's input as a single readable line, sorted
// so the same call always reads the same way.
func compactJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var fields map[string]any

	err := json.Unmarshal(raw, &fields)
	if err != nil {
		return strings.TrimSpace(string(raw))
	}

	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	parts := make([]string, 0, len(keys))

	for _, key := range keys {
		encoded, encErr := json.Marshal(fields[key])
		if encErr != nil {
			continue
		}

		parts = append(parts, key+"="+string(encoded))
	}

	return strings.Join(parts, " ")
}
