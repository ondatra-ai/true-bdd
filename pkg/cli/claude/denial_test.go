package claude_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/cli/claude"
)

func TestDeniedCallsLogTargets(t *testing.T) {
	dir := t.TempDir()

	envelope := `{"type":"result","result":"done","permission_denials":[` +
		`{"tool_name":"Read","tool_input":{"file_path":"/host/file.go"}},` +
		`{"tool_name":"Glob","tool_input":{"pattern":"**/*.go","path":"/host/src"}},` +
		`{"tool_name":"Grep","tool_input":{"pattern":"secret query","path":"/host/tests"}},` +
		`{"tool_name":"Bash","tool_input":{"command":"secret command"}}]}`
	script := "#!/bin/sh\nprintf '%s\\n' '" + envelope + "'\n"

	err := os.WriteFile(filepath.Join(dir, "claude"), []byte(script), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir)

	var logs bytes.Buffer

	previous := slog.Default()

	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	for _, stream := range []bool{false, true} {
		logs.Reset()

		opts := claude.Options{Role: "fix"}

		var err error
		if stream {
			_, err = claude.RunStreamContext(t.Context(), "prompt", opts, func(claude.Event) {})
		} else {
			_, err = claude.RunJSON("prompt", opts)
		}

		if err != nil {
			t.Fatal(err)
		}

		var record map[string]any

		err = json.Unmarshal(logs.Bytes(), &record)
		if err != nil {
			t.Fatal(err)
		}

		want := "Read /host/file.go; Glob /host/src/**/*.go; Grep /host/tests; Bash"
		if record["denied"] != want || record["role"] != "fix" {
			t.Fatalf("stream=%v: %s", stream, logs.String())
		}

		if strings.Contains(logs.String(), "secret") {
			t.Fatal("logged non-target input")
		}
	}
}

func TestDenialTargetsAreBoundedAndOptional(t *testing.T) {
	for _, testCase := range []struct{ name, denials, want string }{
		{"unicode limit",
			`[{"tool_name":"Read","tool_input":{"file_path":"` + strings.Repeat("é", 201) + `"}}]`,
			"Read " + strings.Repeat("é", 200) + "…"},
		{"glob without root", `[{"tool_name":"Glob","tool_input":{"pattern":"**/*.go"}}]`, "Glob **/*.go"},
		{"grep without path", `[{"tool_name":"Grep","tool_input":{"pattern":"private search text"}}]`, "Grep"},
		{"absent input", `[{"tool_name":"Read"}]`, "Read"},
		{"no denials", `[]`, ""},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			dir := t.TempDir()

			script := "#!/bin/sh\ncat <<'RESULT'\n" + `{"result":"done","permission_denials":` + testCase.denials + "}\nRESULT\n"

			err := os.WriteFile(filepath.Join(dir, "claude"), []byte(script), 0o755)
			if err != nil {
				t.Fatal(err)
			}

			t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

			var logs bytes.Buffer

			previous := slog.Default()

			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previous) })

			_, err = claude.RunJSON("prompt", claude.Options{Label: "claude-turn001", Role: "prompt"})
			if err != nil {
				t.Fatal(err)
			}

			if testCase.want == "" {
				if logs.Len() != 0 {
					t.Fatal(logs.String())
				}

				return
			}

			var record map[string]any

			err = json.Unmarshal(logs.Bytes(), &record)
			if err != nil {
				t.Fatal(err)
			}

			if record["denied"] != testCase.want || record["label"] != "claude-turn001" || record["role"] != "prompt" {
				t.Fatal(logs.String())
			}
		})
	}
}
