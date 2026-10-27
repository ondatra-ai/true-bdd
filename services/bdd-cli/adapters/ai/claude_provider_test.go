package ai_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

func TestClaudeWriterGeneratedSettingsAndOutcomes(t *testing.T) {
	for _, testCase := range []struct {
		name, body string
		outcome    pkgerrors.ClaudeTurnOutcome
	}{
		{"success", `printf '%s\n' '{"type":"result","result":"changed"}'`, ""},
		{"exit seven", `printf '%s\n' '{"type":"result","result":"changed"}'; exit 7`, pkgerrors.ClaudeTurnNonZeroExit},
		{"rate limit",
			`printf '%s\n' '{"type":"result","is_error":true,` +
				`"errors":["rate_limit"],"api_error_status":429}'`,
			pkgerrors.ClaudeTurnErrorResult},
		{"empty", `printf '%s\n' '{"type":"result","result":"  "}'`, pkgerrors.ClaudeTurnEmptyOutput},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(root, "state"))

			bin := filepath.Join(root, "bin")

			err := os.Mkdir(bin, 0755)
			if err != nil {
				t.Fatal(err)
			}

			err = os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\n"+testCase.body+"\n"), 0755)
			if err != nil {
				t.Fatal(err)
			}

			goErr := os.WriteFile(filepath.Join(bin, "go"), []byte("#!/bin/sh\nprintf '/toolchain\\n/cache\\n'\n"), 0755)
			if goErr != nil {
				t.Fatal(goErr)
			}

			t.Setenv("PATH", bin)

			req := ai.Request{ClaudeWriter: true, WorkDir: root, TmpDir: filepath.Join(root, "tmp"),
				Label: "claude-turn002", Mode: ai.ExecutionMode{AllowedTools: []string{"Write(./services/**)",
					"Write(./tmp/**)"}, DisallowedTools: []string{"Bash"}}}
			_, err = ai.NewClaudeProvider().Execute(t.Context(), req)

			var failed *pkgerrors.ClaudeTurnError
			if testCase.outcome == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.As(err, &failed) || failed.Outcome != testCase.outcome {
				t.Fatalf("error=%v, want %s", err, testCase.outcome)
			}

			assertClaudeSettings(t, root, req)
			assertClaudePolicy(t, root, req)
		})
	}
}

func TestClaudeStartFailureStaysFatal(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(root, "state"))

	goErr := os.WriteFile(filepath.Join(root, "go"), []byte("#!/bin/sh\nprintf '/toolchain\\n/cache\\n'\n"), 0755)
	if goErr != nil {
		t.Fatal(goErr)
	}

	t.Setenv("PATH", root)
	_, err := ai.NewClaudeProvider().Execute(t.Context(), ai.Request{ClaudeWriter: true, WorkDir: root,
		TmpDir: filepath.Join(root, "tmp"), Label: "claude-turn001"})

	var failed *pkgerrors.ClaudeTurnError
	if err == nil || errors.As(err, &failed) {
		t.Fatalf("start error = %v", err)
	}
}

func assertClaudeSettings(t *testing.T, root string, req ai.Request) {
	t.Helper()

	path := filepath.Join(root, "tmp", req.Label+"-claude-config", "settings.json")

	raw, readErr := disk.Read(path)
	if readErr != nil {
		t.Fatal(readErr)
	}

	var settings map[string]json.RawMessage

	err := json.Unmarshal(raw, &settings)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(settings["hooks"]), "claude-guard --policy") ||
		!strings.Contains(string(settings["env"]), "600000") || strings.Contains(string(settings["permissions"]), "Bash(") {
		t.Fatalf("settings: %s", raw)
	}
}

func assertClaudePolicy(t *testing.T, root string, req ai.Request) {
	t.Helper()

	path := filepath.Join(root, "tmp", req.Label+"-claude-config", "policy.json")

	raw, err := disk.Read(path)
	if err != nil {
		t.Fatal(err)
	}

	var policy ai.ClaudeGuardPolicy

	err = json.Unmarshal(raw, &policy)
	if err != nil {
		t.Fatal(err)
	}

	if len(policy.ReadRoots) != 2 || len(policy.ProtectedRoots) != 2 {
		t.Fatalf("unexpected roots: %+v", policy)
	}

	protected := fmt.Sprintf(`{"tool_name":"Write","cwd":%q,"tool_input":{"file_path":%q}}`, root, path)
	if allowed, reason := policy.DecidePayload([]byte(protected)); allowed {
		t.Fatalf("generated policy writable: %s", reason)
	}

	for _, target := range []string{"/cache/github.com/ondatra-ai/copy/go.mod",
		"/cache/cache/download/github.com/ondatra-ai/copy.zip", "/cache/cache/vcs/repository/config",
		"/Users/owner/.codex/session"} {
		for _, tool := range []string{claudeRead, claudeGlob, claudeGrep} {
			payload := fmt.Sprintf(`{"tool_name":%q,"cwd":%q,"tool_input":{"file_path":%q,"path":%q}}`,
				tool, root, target, target)
			if allowed, reason := policy.DecidePayload([]byte(payload)); allowed {
				t.Errorf("generated policy allowed %s %s: %s", tool, target, reason)
			}
		}
	}
}
