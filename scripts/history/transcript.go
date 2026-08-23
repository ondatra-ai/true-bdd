package history

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
)

// readJSONL reads a transcript, dropping any line that will not parse.
//
// A partial tail is normal — the hook fires while the harness is still
// writing — and the event's own `last_assistant_message` backstops whatever
// the tail was going to say, so a malformed line is skipped rather than
// reported. A missing file is likewise not an error.
//
// Streamed rather than read whole: a live transcript runs to tens of
// megabytes, and one entry carrying a large tool result is far past the
// default scanner's line limit.
func readJSONL(path string) []map[string]any {
	handle, err := os.Open(path) //nolint:gosec // the path comes from the harness's own event.
	if err != nil {
		return nil
	}

	defer handle.Close() //nolint:errcheck // read-only.

	var (
		entries []map[string]any
		reader  = bufio.NewReader(handle)
	)

	for {
		line, err := reader.ReadString('\n')

		if trimmed := strings.TrimSpace(line); trimmed != "" {
			var entry map[string]any
			if json.Unmarshal([]byte(trimmed), &entry) == nil {
				entries = append(entries, entry)
			}
		}

		if err != nil {
			if !errors.Is(err, io.EOF) {
				return entries
			}

			return entries
		}
	}
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
