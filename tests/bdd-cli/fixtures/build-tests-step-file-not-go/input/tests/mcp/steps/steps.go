//go:build bdd

// Package steps binds the mcp-service scenarios' step text to executable
// Go. Its Register body was left half-edited: the last call never closes,
// so the file is not Go at all.
package steps

// StepFunc runs one step against the scenario state.
type StepFunc func(state *State, args []string) error

// Suite is what the scenario runner offers this package.
type Suite interface {
	Step(pattern string, fn StepFunc)
}

// State is the per-scenario state every definition in this package shares.
type State struct{}

// Register installs a definition for every step the scenarios declare.
func Register(suite Suite) {
	suite.Step(`^the MCP server is running$`, noop)
	suite.Step(`^the Claude User opens a session$`, noop)
	suite.Step(`^the session is granted a ticket within (\d+) seconds$`, noop
}

func noop(_ *State, _ []string) error { return nil }
