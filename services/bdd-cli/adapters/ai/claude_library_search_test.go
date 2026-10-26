package ai_test

import (
	"fmt"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
)

func TestClaudeSearchRestrictsAnInHostModuleCache(t *testing.T) {
	t.Parallel()

	policy := ai.ClaudeGuardPolicy{
		Replay:       ai.ReplayPolicy{ReadRoot: claudeHost},
		LibraryCache: "/host/tmp/modcache", ReadRoots: []string{"/host/tmp/modcache/example.com/declared@v1.0.0"}}
	for _, tool := range []string{claudeGrep, claudeGlob} {
		for _, target := range []struct {
			path  string
			allow bool
		}{
			{claudeHost, false}, {"/host/tmp", false}, {policy.LibraryCache, false},
			{"/host/tmp/modcache/example.com/undeclared@v1.0.0", false},
			{"/host/services", true}, {policy.ReadRoots[0], true},
		} {
			payload := fmt.Sprintf(`{"tool_name":%q,"cwd":"/host",`+
				`"tool_input":{"path":%q,"pattern":"**/*.go"}}`, tool, target.path)
			if allowed, reason := policy.DecidePayload([]byte(payload)); allowed != target.allow {
				t.Errorf("%s %s: allowed=%v want=%v (%s)", tool, target.path, allowed, target.allow, reason)
			}
		}
	}
}
