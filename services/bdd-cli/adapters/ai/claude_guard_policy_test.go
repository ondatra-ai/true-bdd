package ai_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
)

const claudeHost = "/host"
const claudeRead = "Read"
const claudeWrite = "Write"
const claudeMultiEdit = "MultiEdit"
const claudeGrep = "Grep"
const claudeGlob = "Glob"
const claudeEdit = "Edit"
const claudeJest = "npx jest --config jest.config.js"
const claudeReplay = "go test ./tests/ -mode=replay"

func TestClaudeWritesResolveDanglingSymlinks(t *testing.T) {
	t.Parallel()

	const (
		escape = "escape"
		inside = "inside"
		cycle  = "cycle"
	)

	host, outside := t.TempDir(), t.TempDir()

	services := filepath.Join(host, "services")

	err := os.Mkdir(services, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	for _, link := range []struct{ name, target string }{
		{escape, filepath.Join(outside, "new.go")},
		{"chain", escape},
		{inside, "new.go"},
		{"inside-chain", inside},
		{cycle, cycle},
	} {
		err := os.Symlink(link.target, filepath.Join(services, link.name))
		if err != nil {
			t.Fatal(err)
		}
	}

	policy := ai.ClaudeGuardPolicy{Replay: ai.ReplayPolicy{ReadRoot: host, WriteRoots: []string{services}}}

	for _, tool := range []string{claudeWrite, claudeEdit, claudeMultiEdit} {
		for _, testCase := range []struct {
			name  string
			allow bool
		}{
			{escape, false}, {"chain", false}, {inside, true}, {"inside-chain", true}, {cycle, false},
		} {
			payload := fmt.Sprintf(`{"tool_name":%q,"cwd":%q,"tool_input":{"file_path":%q}}`,
				tool, host, filepath.Join(services, testCase.name))
			if allowed, reason := policy.DecidePayload([]byte(payload)); allowed != testCase.allow {
				t.Errorf("%s %s: allowed=%v, want %v (%s)", tool, testCase.name, allowed, testCase.allow, reason)
			}
		}
	}
}

func TestClaudeReplayInNestedSuite(t *testing.T) {
	t.Parallel()

	policy := ai.ClaudeGuardPolicy{Replay: ai.ReplayPolicy{ReadRoot: claudeHost,
		TestDir: "/host/tests with 'quotes'", TestCommand: claudeJest, TestSelector: "--testNamePattern"}}

	prefix := `cd '/host/tests with '"'"'quotes'"'"'' && `
	for _, testCase := range []struct {
		command, cwd string
		allow        bool
	}{
		{prefix + claudeJest, claudeHost, true},
		{prefix + "npx jest --config jest.config.js --testNamePattern '^works$'", claudeHost, true},
		{prefix + "npx jest --config jest.config.js --testNamePattern '^works$'", policy.Replay.TestDir, true},
		{claudeJest, policy.Replay.TestDir, true},
		{claudeJest, claudeHost, false},
		{prefix + claudeJest, "/other", false},
		{prefix + "npx jest --config jest.config.js; pwd", claudeHost, false},
		{prefix + "npx jest --config jest.config.js && pwd", claudeHost, false},
		{prefix + "npx jest --config jest.config.js --testNamePattern ''", claudeHost, false},
		{"cd /host/tests && npx jest --config jest.config.js", claudeHost, false},
		{prefix + "cd /other && npx jest --config jest.config.js", claudeHost, false},
	} {
		payload := fmt.Sprintf(`{"tool_name":"Bash","cwd":%q,"tool_input":{"command":%q}}`, testCase.cwd, testCase.command)
		if allowed, reason := policy.DecidePayload([]byte(payload)); allowed != testCase.allow {
			t.Errorf("%s: allowed=%v, want %v (%s)", payload, allowed, testCase.allow, reason)
		}
	}
}

func TestClaudeFileToolsUseDirectoryAfterReplay(t *testing.T) {
	t.Parallel()

	policy := ai.ClaudeGuardPolicy{Replay: ai.ReplayPolicy{ReadRoot: claudeHost,
		WriteRoots: []string{"/host/services"}}}

	for _, target := range []string{"services/forbidden.go", "../services/allowed.go"} {
		payload := fmt.Sprintf(`{"tool_name":"Write","cwd":"/host/tests","tool_input":{"file_path":%q}}`, target)

		want := target == "../services/allowed.go"
		if allowed, reason := policy.DecidePayload([]byte(payload)); allowed != want {
			t.Errorf("file target %s after nested replay: allowed=%v want=%v (%s)", target, allowed, want, reason)
		}
	}
}

func TestClaudeTildePathsCannotEscape(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	cache := filepath.Join(home, "go/pkg/mod")
	policy := ai.ClaudeGuardPolicy{Replay: ai.ReplayPolicy{ReadRoot: claudeHost,
		WriteRoots: []string{claudeHost}}, ReadRoots: []string{cache},
		ExcludedRoots: []string{filepath.Join(cache, "github.com/ondatra-ai")}}

	for _, tool := range []string{claudeRead, claudeGlob, claudeGrep, claudeEdit, claudeWrite, claudeMultiEdit} {
		for _, target := range []string{"~", "~/go/pkg/mod/github.com/ondatra-ai", "~/go/pkg/mod"} {
			for _, field := range []string{"file_path", claudePathKey} {
				raw, marshalErr := json.Marshal(map[string]any{"tool_name": tool, "cwd": claudeHost,
					"tool_input": map[string]string{field: target}})
				if marshalErr != nil {
					t.Fatal(marshalErr)
				}
				// Read may inspect the cache directory itself; searches must reject its excluded descendants.
				want := tool == claudeRead && target == "~/go/pkg/mod"
				if allowed, reason := policy.DecidePayload(raw); allowed != want {
					t.Errorf("%s %s=%s: allowed=%v, want %v (%s)", tool, field, target, allowed, want, reason)
				}
			}
		}
	}

	for _, pattern := range []string{"~", "~/go/pkg/mod/github.com/ondatra-ai/**", "~/go/pkg/mod/**"} {
		payload := fmt.Sprintf(`{"tool_name":"Glob","cwd":"/host","tool_input":{"path":"/host","pattern":%q}}`, pattern)
		if allowed, reason := policy.DecidePayload([]byte(payload)); allowed {
			t.Errorf("tilde glob escaped: %s (%s)", payload, reason)
		}
	}
}

func TestClaudeHookCanonicalReplay(t *testing.T) {
	t.Parallel()

	policy := ai.ClaudeGuardPolicy{Replay: ai.ReplayPolicy{ReadRoot: claudeHost,
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

	policy := ai.ClaudeGuardPolicy{Replay: ai.ReplayPolicy{ReadRoot: host,
		WriteRoots: []string{filepath.Join(host, "services"), filepath.Join(host, "tmp")}},
		ReadRoots: []string{"/toolchain", "/cache", "/own-spills"}, ExcludedRoots: []string{"/cache/github.com/ondatra-ai"}}
	for _, tool := range []string{claudeRead, claudeGlob, claudeGrep, claudeEdit, claudeWrite, claudeMultiEdit} {
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

	policy := ai.ClaudeGuardPolicy{Replay: ai.ReplayPolicy{ReadRoot: claudeHost},
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

	policy := ai.ClaudeGuardPolicy{Replay: ai.ReplayPolicy{ReadRoot: claudeHost, WriteRoots: []string{"/host/tmp"}},
		ProtectedRoots: []string{"/host/tmp/claude-config"}}

	for _, tool := range []string{claudeWrite, claudeEdit, claudeMultiEdit} {
		payload := fmt.Sprintf(
			`{"tool_name":%q,"cwd":"/host","tool_input":{"file_path":"/host/tmp/claude-config/policy.json"}}`, tool)
		if allowed, reason := policy.DecidePayload([]byte(payload)); allowed {
			t.Fatalf("%s widened its own guard: %s", tool, reason)
		}
	}
}
