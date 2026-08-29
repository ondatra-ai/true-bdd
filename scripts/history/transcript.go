package history

import (
	"encoding/json"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// readJSONL reads a transcript, dropping any line that will not parse. A
// partial tail is normal: the hook fires mid-write, and
// last_assistant_message backstops it.
func readJSONL(path string) []map[string]any {
	raw, err := disk.Read(path)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(raw), "\n")
	entries := make([]map[string]any, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		var entry map[string]any
		if json.Unmarshal([]byte(trimmed), &entry) == nil {
			entries = append(entries, entry)
		}
	}

	return entries
}

// content is an entry's content, from `message.content` or the entry itself.
func content(entry map[string]any) any {
	if message, ok := entry["message"].(map[string]any); ok {
		if inner, present := message["content"]; present && inner != nil {
			return inner
		}
	}

	return entry["content"]
}

// isPrompt reports whether an entry carries text a human actually typed.
func isPrompt(entry map[string]any) bool {
	switch typed := content(entry).(type) {
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		for _, block := range typed {
			if _, ok := textOf(block); ok {
				return true
			}
		}

		return false
	default:
		return false
	}
}

// textOf returns a content block's text, and whether it had any.
func textOf(block any) (string, bool) {
	mapped, ok := block.(map[string]any)
	if !ok {
		return "", false
	}

	if kind, _ := mapped["type"].(string); kind != "text" {
		return "", false
	}

	text, _ := mapped["text"].(string)

	trimmed := strings.TrimSpace(text)

	return trimmed, trimmed != ""
}

// skip drops the entries that are not part of the visible conversation.
func skip(entry map[string]any) bool {
	return truthy(entry["isMeta"]) || truthy(entry["isSidechain"])
}

// turnBlocks is every assistant text block after the last user prompt, in
// order.
func turnBlocks(entries []map[string]any) []string {
	start := 0

	for index, entry := range entries {
		if skip(entry) {
			continue
		}

		if kind, _ := entry["type"].(string); kind == "user" && isPrompt(entry) {
			start = index + 1
		}
	}

	var blocks []string

	for _, entry := range entries[start:] {
		if skip(entry) {
			continue
		}

		if kind, _ := entry["type"].(string); kind != "assistant" {
			continue
		}

		blocks = append(blocks, assistantText(content(entry))...)
	}

	return blocks
}

// assistantText pulls the text out of one assistant entry's content, which is
// either a bare string or a list of blocks.
func assistantText(raw any) []string {
	switch typed := raw.(type) {
	case string:
		if trimmed := strings.TrimSpace(typed); trimmed != "" {
			return []string{trimmed}
		}

		return nil
	case []any:
		blocks := make([]string, 0, len(typed))

		for _, block := range typed {
			if text, ok := textOf(block); ok {
				blocks = append(blocks, text)
			}
		}

		return blocks
	default:
		return nil
	}
}
