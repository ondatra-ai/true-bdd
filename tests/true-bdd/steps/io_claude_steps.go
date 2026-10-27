package steps

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

func registerIOClaudeSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the engine uses an observed fixture Claude CLI$`, observeFixtureClaude)
}

// Prepending only Claude leaves codex behind the existing lifecycle-observing proxy.
func observeFixtureClaude(state *State, _ []string) error {
	fixture, err := filepath.Abs(state.Fixture.Dir)
	if err != nil {
		return state.fail("resolve Claude fixture: %w", err)
	}

	childPath := os.Getenv("PATH")

	for _, entry := range state.Proxy.Env {
		if strings.HasPrefix(entry, "PATH=") {
			childPath = strings.TrimPrefix(entry, "PATH=")
		}
	}

	state.Proxy.Env = append(state.Proxy.Env,
		"PATH="+filepath.Join(fixture, "claude-only")+string(os.PathListSeparator)+childPath,
		"CLAUDE_CONFIG_DIR="+filepath.Join(state.Harness.SessionRoot, state.Fixture.Name, "tmp/claude-state"),
		"TRUE_BDD_IO_FIXTURE="+fixture, "CLAUDECODE=fixture-parent-session", "CLAUDE_HISTORY_ROLE=inherited-parent-role",
		"CLAUDE_CODE_SAFE_MODE=1", "CLAUDE_CODE_SIMPLE=1", "CLAUDE_RELAUNCH_SESSION_ADD_DIRS=/outside/inherited",
		"CLAUDE_CODE_DISABLE_ADVISOR_TOOL=0")

	return nil
}
