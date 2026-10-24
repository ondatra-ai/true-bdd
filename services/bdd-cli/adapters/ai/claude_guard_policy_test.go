package ai_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
)

const claudeHost = "/host"
const claudeReplay = "go test ./tests/ -mode=replay"

func TestClaudeHookCanonicalReplay(t *testing.T) {
	t.Parallel()

	policy := ai.ClaudeGuardPolicy{Replay: ai.CrushGuardPolicy{ReadRoot: claudeHost,
		TestCommand: claudeReplay, TestSelector: "-" + "run", TestDir: claudeHost}}

	for _, testCase := range []struct {
		command, cwd      string
		background, allow bool
	}{
		{claudeReplay, claudeHost, false, true},
		{"go test ./tests/ -mode=replay -run '^TestOne$'", claudeHost, false, true},
		{"go test ./tests/ -mode=replay; pwd", claudeHost, false, false},
		{"go test ./tests/ -mode=replay -run \"TestOne\"", claudeHost, false, false},
		{claudeReplay, claudeHost, true, false},
		{claudeReplay, "/other", false, false},
	} {
		payload := fmt.Sprintf(`{"tool_name":"Bash","cwd":%q,"tool_input":{"command":%q,"run_in_background":%v}}`,
			testCase.cwd, testCase.command, testCase.background)

		allowed, reason := policy.DecidePayload([]byte(payload))
		if allowed != testCase.allow || (!allowed && reason == "") {
			t.Errorf("%s: allowed=%v reason=%q", payload, allowed, reason)
		}
	}
}

func TestClaudeFileToolsCannotEscape(t *testing.T) {
	t.Parallel()
	host := t.TempDir()

	outside := t.TempDir()

	err := os.Symlink(outside, filepath.Join(host, "escape"))
	if err != nil {
		t.Fatal(err)
	}

	policy := ai.ClaudeGuardPolicy{Replay: ai.CrushGuardPolicy{ReadRoot: host,
		WriteRoots: []string{filepath.Join(host, "services"), filepath.Join(host, "tmp")}},
		ReadRoots: []string{"/toolchain", "/cache", "/own-spills"}, ExcludedRoots: []string{"/cache/github.com/ondatra-ai"}}
	for _, tool := range []string{"Read", "Glob", "Grep", "Edit", "Write", "MultiEdit"} {
		for _, target := range []string{outside, "/Users/owner/work/other", "/Users/owner/.claude/projects/other",
			"/Users/owner/.codex", "/private/tmp/other", "/cache/github.com/ondatra-ai/copy", filepath.Join(host, "escape",
				"marker")} {
			payload := fmt.Sprintf(`{"tool_name":%q,"cwd":%q,"tool_input":{"file_path":%q,"path":%q}}`, tool, host, target,
				target)
			if allow, reason := policy.DecidePayload([]byte(payload)); allow {
				t.Errorf("%s %s escaped: %s", tool, target, reason)
			}
		}
	}

	for _, target := range []string{host, "/toolchain/src/fmt/print.go", "/own-spills/result.txt"} {
		payload := fmt.Sprintf(`{"tool_name":"Read","cwd":%q,"tool_input":{"file_path":%q}}`, host, target)
		if allow, reason := policy.DecidePayload([]byte(payload)); !allow {
			t.Errorf("denied %s: %s", target, reason)
		}
	}
}

func TestClaudeSearchCannotReachExcludedModuleCopies(t *testing.T) {
	t.Parallel()

	policy := ai.ClaudeGuardPolicy{Replay: ai.CrushGuardPolicy{ReadRoot: claudeHost},
		ReadRoots: []string{"/cache"}, ExcludedRoots: []string{"/cache/github.com/ondatra-ai"}}
	for _, payload := range []string{
		`{"tool_name":"Grep","cwd":"/host","tool_input":{"path":"/cache","pattern":"marker"}}`,
		`{"tool_name":"Glob","cwd":"/host","tool_input":{"path":"/cache","pattern":"github.com/ondatra-ai/**"}}`,
		`{"tool_name":"Glob","cwd":"/host","tool_input":{"path":"/host","pattern":"/cache/github.com/ondatra-ai/**"}}`,
		`{"tool_name":"Glob","cwd":"/host","tool_input":{"path":"/host","pattern":"*/../../outside/marker"}}`,
	} {
		if allowed, reason := policy.DecidePayload([]byte(payload)); allowed {
			t.Errorf("escaped search %s: %s", payload, reason)
		}
	}

	payload := `{"tool_name":"Glob","cwd":"/host","tool_input":{"path":"/cache/github.com/spf13","pattern":"**/*.go"}}`
	if allowed, reason := policy.DecidePayload([]byte(payload)); !allowed {
		t.Fatal(reason)
	}
}

func TestClaudeWriterCannotChangeItsGuard(t *testing.T) {
	t.Parallel()

	policy := ai.ClaudeGuardPolicy{Replay: ai.CrushGuardPolicy{ReadRoot: claudeHost, WriteRoots: []string{"/host/tmp"}},
		ProtectedRoots: []string{"/host/tmp/claude-config"}}

	for _, tool := range []string{"Write", "Edit", "MultiEdit"} {
		payload := fmt.Sprintf(
			`{"tool_name":%q,"cwd":"/host","tool_input":{"file_path":"/host/tmp/claude-config/policy.json"}}`, tool)
		if allowed, reason := policy.DecidePayload([]byte(payload)); allowed {
			t.Fatalf("%s widened its own guard: %s", tool, reason)
		}
	}
}
