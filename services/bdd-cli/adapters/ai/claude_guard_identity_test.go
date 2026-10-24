package ai_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
)

const claudePathKey = "path"

func TestClaudeFileToolsUseIdentity(t *testing.T) {
	t.Parallel()
	policy := claudeIdentityPolicy(t)
	host := policy.Replay.ReadRoot
	alias := filepath.Join(filepath.Dir(host), "hOST")
	cache := policy.ReadRoots[0]

	for _, tool := range []string{claudeRead, claudeGlob, claudeGrep, claudeWrite, claudeEdit, claudeMultiEdit} {
		reading := tool == claudeRead || tool == claudeGlob || tool == claudeGrep
		for _, testCase := range []struct {
			target string
			allow  bool
		}{
			{filepath.Join(alias, "SERVICES", "Marker"), true},
			{filepath.Join(alias, "SERVICES", "new", "file"), true},
			{filepath.Join(cache, "eXCLUDED", "Marker"), false},
			{filepath.Join(cache, "eXCLUDED", "new", "file"), false},
			{alias + "/escape/../Excluded/Marker", false},
			{filepath.Join(host, "tmp", "pOLICY.JSON"), reading},
			{filepath.Join(filepath.Dir(host), "cACHE", "Dependency", "new"), reading},
		} {
			t.Run(tool+testCase.target, func(t *testing.T) {
                t.Parallel()
				assertClaudeIdentityCall(t, policy, tool, host, map[string]string{
					"file_path": testCase.target, claudePathKey: testCase.target}, testCase.allow)
			})
		}
	}
}

func TestClaudeSearchRootsUseIdentity(t *testing.T) {
	t.Parallel()
	policy := claudeIdentityPolicy(t)
	host := policy.Replay.ReadRoot

	cacheAlias := filepath.Join(filepath.Dir(host), "cACHE")
	for _, tool := range []string{claudeGlob, claudeGrep} {
		assertClaudeIdentityCall(t, policy, tool, host, map[string]string{
			claudePathKey: cacheAlias + "/", "pattern": "**"}, false)
	}

	assertClaudeIdentityCall(t, policy, claudeGlob, host, map[string]string{
		claudePathKey: host, "pattern": filepath.Join(cacheAlias, "eXCLUDED", "*")}, false)
}

func claudeIdentityPolicy(t *testing.T) ai.ClaudeGuardPolicy {
	t.Helper()
	root := t.TempDir()
	host := filepath.Join(root, "Host")
	cache := filepath.Join(root, "Cache")
	excluded := filepath.Join(cache, "Excluded")

	protected := filepath.Join(host, "tmp", "Policy.json")
	for _, path := range []string{filepath.Join(host, "services", "Marker"),
		filepath.Join(excluded, "Marker"), filepath.Join(cache, "Dependency", "Marker"), protected} {
		err := disk.Write(path, []byte("marker"), disk.Private)
		if err != nil {
			t.Fatal(err)
		}
	}

	requireClaudeCaseAlias(t, filepath.Join(root, "hOST"))

	err := os.Symlink(excluded, filepath.Join(host, "escape"))
	if err != nil {
		t.Fatal(err)
	}

	return ai.ClaudeGuardPolicy{Replay: ai.CrushGuardPolicy{ReadRoot: host,
		WriteRoots: []string{filepath.Join(host, "services"), filepath.Join(host, "tmp")}},
		ReadRoots: []string{cache}, ExcludedRoots: []string{excluded}, ProtectedRoots: []string{protected}}
}

func requireClaudeCaseAlias(t *testing.T, alias string) {
	t.Helper()

	_, err := os.Stat(alias)
	if err != nil {
		t.Skip("case aliases require a case-insensitive volume")
	}
}

func assertClaudeIdentityCall(t *testing.T, policy ai.ClaudeGuardPolicy, tool, cwd string,
	input map[string]string, want bool) {
	t.Helper()

	payload := struct {
		Tool  string            `json:"tool_name"`
		Cwd   string            `json:"cwd"`
		Input map[string]string `json:"tool_input"`
	}{tool, cwd, input}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	if allowed, reason := policy.DecidePayload(raw); allowed != want {
		t.Fatalf("%s: allowed=%v want=%v: %s", raw, allowed, want, reason)
	}
}

func TestClaudeSuiteDirectoryUsesIdentity(t *testing.T) {
	t.Parallel()
	host := t.TempDir()
	suite := filepath.Join(host, "Suite")

	err := disk.Dir(suite, disk.Private)
	if err != nil {
		t.Fatal(err)
	}

	alias := filepath.Join(host, "sUITE")
	requireClaudeCaseAlias(t, alias)

	policy := ai.ClaudeGuardPolicy{Replay: ai.CrushGuardPolicy{ReadRoot: host, TestDir: suite,
		TestCommand: claudeReplay}}
	assertClaudeIdentityCall(t, policy, "Bash", alias+"/../sUITE/",
		map[string]string{"command": claudeReplay}, true)
}
