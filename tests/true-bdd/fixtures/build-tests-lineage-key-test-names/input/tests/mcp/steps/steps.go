//go:build bdd

// Package steps binds the mcp-service scenarios' step text to executable
// Go. Nothing compiles it here — the sentinel go.mod fences the tree off
// and `build tests` only parses it to learn which patterns are registered.
package steps

// httpStatus is spliced into a pattern rather than written inline, so
// this fixture exercises the scanner's constant folding and not only its
// string literals.
const httpStatus = `(\d{3})`

// StepFunc runs one step against the scenario state.
type StepFunc func(state *State, args []string) error

// Suite is what the scenario runner offers this package.
type Suite interface {
	Step(pattern string, fn StepFunc)
}

// State is the per-scenario state every definition in this package shares.
type State struct{}

// Register installs a definition for every step both scenarios declare.
func Register(suite Suite) {
	suite.Step(`^the MCP server is running on its configured port$`, noop)
	suite.Step(`^the Claude User posts a valid JSON-RPC initialize request to /mcp$`, noop)
	suite.Step(`^the Claude User posts a malformed JSON-RPC request to /mcp$`, noop)
	suite.Step(`^the server returns HTTP `+httpStatus+`$`, noop)
}

func noop(_ *State, _ []string) error { return nil }
