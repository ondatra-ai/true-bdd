package steps

import (
	"encoding/json"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

func registerClaudeGuardSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^claude's write policy grants the test command "([^"]+)" with selector "([^"]+)" in "([^"]+)"$`,
		grantTestCommand)
	suite.Step(`^claude pipes a "(foreground|background)" shell call in directory "([^"]+)" with command (.+)$`,
		pipeClaudeShell)
	suite.Step(`^claude asks the guard about the tool call$`, askClaudeGuard)
}

func pipeClaudeShell(state *State, args []string) error {
	call := state.guard()
	call.background = args[0] == "background"
	call.workingDir, call.command = args[1], args[2]

	return nil
}

func askClaudeGuard(state *State, _ []string) error {
	if state.Fixture == nil {
		return state.fail("%w", ErrNoTreePrepared)
	}

	dir := runner.RunDir(state.Harness.SessionRoot, state.Fixture.Name)
	call := state.guard()

	replay, err := call.policy(dir)
	if err != nil {
		return state.fail("claude replay policy: %w", err)
	}

	policy := `{"replay":` + replay + `}`
	state.Fixture.PrepCmds = append(state.Fixture.PrepCmds,
		"mkdir -p tmp && printf '%s' "+shellQuote(policy)+" > tmp/claude-guard-policy.json")

	cwd := call.workingDir
	if !filepath.IsAbs(cwd) {
		cwd = filepath.Join(dir, cwd)
	}

	payload, err := json.Marshal(map[string]any{"tool_name": "Bash", "cwd": cwd,
		"tool_input": map[string]any{"command": call.command, "run_in_background": call.background}})
	if err != nil {
		return state.fail("claude hook payload: %w", err)
	}

	state.Fixture.Stdin = payload

	return runEngine(state, "claude-guard --policy "+"tmp/claude-guard-policy.json")
}
