//go:build bdd

// Package steps binds the mcp-service scenarios' step text to executable
// Go. Nothing compiles it here — the sentinel go.mod fences the tree off
// and `build tests` only parses it to learn which patterns are registered.
package steps

import "fmt"

// httpStatus is spliced into the pattern below through a CALL, so what
// the suite would register is not derivable from this source.
const httpStatus = `(\d{3})`

// StepFunc runs one step against the scenario state.
type StepFunc func(state *State, args []string) error

// Suite is what the scenario runner offers this package.
type Suite interface {
	Step(pattern string, fn StepFunc)
}

// State is the per-scenario state every definition in this package shares.
type State struct{}

// Register installs a definition for every step the scenario declares —
// the last one through fmt.Sprintf, which the engine must refuse.
func Register(suite Suite) {
	suite.Step(`^the MCP server is running on its configured port$`, noop)
	suite.Step(`^the Claude User posts a valid JSON-RPC initialize request to /mcp$`, noop)
	suite.Step(fmt.Sprintf(`^the server returns HTTP %s$`, httpStatus), noop)
}

func noop(_ *State, _ []string) error { return nil }
