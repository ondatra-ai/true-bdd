package main

import (
	"bytes"
	"encoding/json"
)

// initBallastFields are the `system`/`init` frame's inventories of the
// RECORDING MACHINE — tool schemas, connected MCP servers, installed
// skills/plugins — dropped since a public repo shouldn't carry a developer's integrations.
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
// stream-json stdout, leaving all other lines byte-identical: a line it
// cannot confidently parse as JSON is passed through untouched, not rewritten.
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
// uninteresting one is left exactly as it arrived rather than re-encoded
// (re-encoding would reorder keys and make every cassette diff noise).
func sanitizeLine(line []byte) ([]byte, bool) {
	// Cheap reject on the FIELD NAME only, never the whole `"type":"system"`
	// pair: matching that as raw bytes is whitespace-sensitive, and a frame
	// serialised with a space after the colon would sail through with its inventory intact.
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
