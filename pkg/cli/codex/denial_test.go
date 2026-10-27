package codex_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
 "strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/cli/codex"
)

func TestReportedRefusalsLogTargets(t *testing.T) {
	var logs bytes.Buffer

	previous := slog.Default()

	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	events := strings.Join([]string{
		`{"type":"item.started","item":{"type":"command_execution","command":"cat /outside/file",` +
			`"status":"in_progress"}}`,
		`{"type":"item.completed","item":{"type":"command_execution","command":"cat /outside/file",` +
			`"status":"failed","aggregated_output":"cat: /outside/file: Permission denied"}}`,
		`{"type":"item.completed","item":{"type":"file_change",` +
			`"status":"declined","changes":[{"path":"/outside/edit.go","kind":"update"}]}}`,
		`{"type":"item.completed","item":{"type":"command_execution","command":"go test ./...",` +
			`"status":"failed","aggregated_output":"assertion failed"}}`,
		`{"type":"item.completed","item":{"type":"command_execution","command":"echo permission denied",` +
			`"status":"completed","aggregated_output":"permission denied"}}`,
		`{"type":"item.completed","item":{"type":"mcp_tool_call","tool":"Read","status":"failed",` +
			`"arguments":{"file_path":"/host/private.go"},"error":{"message":"Permission denied"}}}`,
		`{"type":"item.completed","item":{"type":"command_execution","status":"declined",` +
			`"command":"/bin/zsh -c 'cat \"/outside/with spaces/file.go\"'"}}`,
		`{"type":"item.completed","item":{"type":"command_execution","status":"declined",` +
			`"command":"cat /bin/bash"}}`,
		`not JSON`,
		`{"type":"item.completed","item":{"type":"command_execution","command":"ls /private","status":"declined"}}`,
		`{"type":"item.completed","item":{"type":"command_execution","command":"pwd","status":"declined"}}`,
	}, "\n")
	codex.WarnDenials(events, "codex-turn002", "apply")

	var record map[string]any

	err := json.Unmarshal(logs.Bytes(), &record)
	if err != nil {
		t.Fatal(err)
	}

	want := "command_execution /outside/file; file_change /outside/edit.go; Read /host/private.go; " +
		"command_execution /outside/with spaces/file.go; command_execution /bin/bash; " +
		"command_execution /private; command_execution"
	if record["label"] != "codex-turn002" || record["role"] != "apply" || record["denied"] != want {
		t.Fatal(logs.String())
	}
}
