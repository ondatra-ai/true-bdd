//go:build bdd

// Package steps binds the mcp-service scenarios' step text to executable
// Go. Nothing compiles it here — the sentinel go.mod fences the tree off
// and `build tests` only parses it to learn which patterns are registered.
package steps

// StepFunc runs one step against the scenario state.
type StepFunc func(state *State, args []string) error

// Suite is what the scenario runner offers this package.
type Suite interface {
	Step(pattern string, fn StepFunc)
}

// State is the per-scenario state every definition in this package shares.
type State struct{}

// Register installs two definitions that both match the scenario's Then
// step, which is what the engine must refuse.
func Register(suite Suite) {
	suite.Step(`^the MCP server is running on its configured port$`, noop)
	suite.Step(`^the Claude User posts a valid JSON-RPC initialize request to /mcp$`, noop)
	suite.Step(`^the server returns HTTP (\d{3})$`, noop)
	suite.Step(`^the server returns HTTP (.+)$`, noop)
}

func noop(_ *State, _ []string) error { return nil }
