package steps

import (
	"encoding/json"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

func registerClaudeGuardSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^claude's write policy lets the turn write under "([^"]+)"$`, grantWriteRoot)
	suite.Step(`^claude's write policy is missing$`, withholdPolicy)
	suite.Step(`^claude's write policy is unreadable$`, garblePolicy)
	suite.Step(`^claude's write policy names no directory to read$`, omitReadRoot)
	suite.Step(`^the hook payload names the tool "([^"]+)" and the path "([^"]+)"$`, pipePathPayload)
	suite.Step(`^the tool call names the tool "([^"]+)"$`, nameTool)
	suite.Step(`^the tool call names the file "([^"]+)" of the project tree$`, nameTreeFile)
	suite.Step(`^the tool call names the path "([^"]+)" as written$`, nameLiteralPath)
	suite.Step(`^the hook payload names the tool "([^"]+)" and the file "([^"]+)"$`, pipePayload)

	suite.Step(`^claude's write policy grants the test command "([^"]+)" with selector "([^"]+)" in "([^"]+)"$`,
		grantTestCommand)
	suite.Step(`^the tool call is a "(foreground|background)" shell call in directory "([^"]+)" with command (.+)$`,
		pipeClaudeShell)
	suite.Step(`^claude asks the guard about the tool call$`, askClaudeGuard)
}

//truebdd:concept given "tool call"
func pipeClaudeShell(state *State, args []string) error {
	call := state.guard()
	call.background = args[0] == "background"
	call.workingDir, call.command = args[1], args[2]

	return nil
}

//truebdd:concept when "the write guard is asked"
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
	if call.unreadable {
		policy = "invalid"
	}

	if call.withheld {
		policy = "{}"
	}

	if !call.withheld {
		state.Fixture.PrepCmds = append(state.Fixture.PrepCmds,
			"mkdir -p tmp && printf '%s' "+shellQuote(policy)+" > tmp/claude-guard-policy.json")
	}

	cwd := call.workingDir
	if !filepath.IsAbs(cwd) {
		cwd = filepath.Join(dir, cwd)
	}

	tool, target, path := call.claudeTarget(dir)

	payload, err := json.Marshal(map[string]any{"tool_name": tool, "cwd": cwd,
		"tool_input": map[string]any{"command": call.command, "run_in_background": call.background,
			"file_path": target, "path": path}})
	if err != nil {
		return state.fail("claude hook payload: %w", err)
	}

	state.Fixture.Stdin = payload

	return runEngine(state, "claude-guard --policy "+"tmp/claude-guard-policy.json")
}

func (call *guardCall) claudeTarget(dir string) (string, string, string) {
	tool := call.tool
	target := call.targetPath(dir)
	path := ""

	if call.piped {
		tool = call.pipedTool
		target = filepath.Join(dir, call.pipedFile)
		path = call.pipedPath
	}

	if tool == "" {
		tool = "Bash"
	}

	if path != "" {
		target = ""
	}

	return tool, target, path
}
