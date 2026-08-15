package main

import (
	"bytes"
	"encoding/json"
)

// initBallastFields are the `system`/`init` frame's inventories of the
// RECORDING MACHINE: every tool schema, every connected MCP server with
// its auth status, the installed skills, plugins and slash commands, and
// the paths their caches live at.
//
// They are dropped before a cassette is written, for two reasons that
// point the same way. They are personal: a cassette is committed, and a
// public repository has no business carrying which integrations a
// developer has connected. And they are noise: the engine's stream
// handler answers a system message with two slog.Debug calls
// (adapters/ai/claude_provider.go) and reads nothing out of it, while
// these fields are 47% of every recording's bytes.
//
// What stays is the protocol: type, subtype, session_id, model, and the
// normalized cwd — enough for a reader to see which call this was.
func initBallastFields() []string {
	return []string{
		"tools",
		"mcp_servers",
		"skills",
		"plugins",
		"slash_commands",
		"agents",
		"memory_paths",
		"commands",
		"apiKeySource",
		"output_style",
	}
}

// sanitizeStream strips the ballast from every `system` frame in a
// stream-json stdout, leaving all other lines byte-identical.
//
// Line-oriented and forgiving on purpose: a line that is not JSON, or
// not an object, is passed through untouched. The shim's job is to
// reproduce what the CLI said, so anything it cannot confidently parse
// it must not rewrite.
func sanitizeStream(stdout []byte) []byte {
	lines := bytes.Split(stdout, []byte("\n"))

	for index, line := range lines {
		cleaned, ok := sanitizeLine(line)
		if ok {
			lines[index] = cleaned
		}
	}

	return bytes.Join(lines, []byte("\n"))
}

// sanitizeLine reports whether it rewrote the line, so an unparseable or
// uninteresting one is left exactly as it arrived rather than
// re-encoded — re-encoding would reorder keys and make every cassette
// diff noise.
func sanitizeLine(line []byte) ([]byte, bool) {
	// Cheap reject first — most lines are assistant deltas — but only on
	// the FIELD NAME, never on the whole `"type":"system"` pair: matching
	// that as raw bytes makes the sanitizer whitespace-sensitive, and a
	// frame serialised with a space after the colon would sail through
	// carrying the very inventory this exists to strip.
	if !bytes.Contains(line, []byte(`"type"`)) {
		return nil, false
	}

	var frame map[string]any

	err := json.Unmarshal(line, &frame)
	if err != nil {
		return nil, false
	}

	if frameType, _ := frame["type"].(string); frameType != "system" {
		return nil, false
	}

	dropped := false

	for _, field := range initBallastFields() {
		if _, present := frame[field]; present {
			delete(frame, field)

			dropped = true
		}
	}

	if !dropped {
		return nil, false
	}

	encoded, err := json.Marshal(frame)
	if err != nil {
		return nil, false
	}

	return encoded, true
}
