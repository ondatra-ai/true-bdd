package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The frame the CLI opens every session with, trimmed to the shape that
// matters: protocol fields plus the machine inventory.
const initFrame = `{"type":"system","subtype":"init","session_id":"abc","model":"claude-opus-4-8",` +
	`"cwd":"{{CWD}}","tools":["Read","Write","Bash"],` +
	`"mcp_servers":[{"name":"claude.ai GMail","status":"connected"}],` +
	`"skills":["a"],"plugins":["b"],"slash_commands":["c"],` +
	`"memory_paths":{"auto":"{{HOME}}/work/notes.md"}}`

func TestSanitizeStreamDropsMachineInventory(t *testing.T) {
	t.Parallel()

	out := sanitizeStream([]byte(initFrame))

	var frame map[string]any

	err := json.Unmarshal(out, &frame)
	if err != nil {
		t.Fatalf("sanitized frame is not JSON: %v", err)
	}

	for _, gone := range initBallastFields() {
		if _, present := frame[gone]; present {
			t.Errorf("%q survived sanitizing", gone)
		}
	}

	// The protocol must survive: a cassette that cannot say which call it
	// was is not a recording, it is a redaction.
	for _, kept := range []string{"type", "subtype", "session_id", "model", "cwd"} {
		if _, present := frame[kept]; !present {
			t.Errorf("%q was dropped but is protocol, not inventory", kept)
		}
	}
}

// The reason the type check moved after json.Unmarshal: a frame
// serialised with a space after the colon is the same frame, and a
// byte-match on `"type":"system"` would wave it through with its
// inventory intact.
func TestSanitizeStreamIsNotWhitespaceSensitive(t *testing.T) {
	t.Parallel()

	spaced := `{"type": "system", "subtype": "init", "tools": ["Read"], "session_id": "x"}`

	out := string(sanitizeStream([]byte(spaced)))
	if strings.Contains(out, "tools") {
		t.Fatalf("inventory survived a whitespace-formatted frame: %s", out)
	}
}

func TestSanitizeStreamLeavesEverythingElseByteIdentical(t *testing.T) {
	t.Parallel()

	// An assistant delta, a non-init system frame with nothing to drop,
	// a plain-text line, and an empty line.
	stream := strings.Join([]string{
		`{"type":"assistant","message":{"content":"hello"}}`,
		`{"type":"system","subtype":"hook_started","hook_name":"SessionStart"}`,
		`not json at all`,
		``,
	}, "\n")

	if out := string(sanitizeStream([]byte(stream))); out != stream {
		t.Fatalf("sanitizer rewrote lines it should not have:\nwant %q\ngot  %q", stream, out)
	}
}
