//go:build bdd

// Package steps binds the mcp-service scenarios' step text to executable
// Go. Nothing compiles it here: the sentinel go.mod fences the tree off,
// and `scen check` only parses it to learn which patterns are registered.
package steps

// StepFunc runs one step against the scenario state.
type StepFunc func(state *State, args []string) error

// Suite is what the scenario runner offers this package.
type Suite interface {
	Step(pattern string, fn StepFunc)
}

// State is the per-scenario state every definition in this package shares.
type State struct{}

// Register installs a second definition for the Operator's ping, so that
// step binds to two definitions, which the suite itself would refuse.
func Register(suite Suite) {
	suite.Step(`^the mcp service is running$`, noop)
	suite.Step(`^the (Operator|Client Developer) sends "([^"]+)"$`, noop)
	suite.Step(`^the Operator sends "ping"$`, noop)
	suite.Step(`^the service answers "([^"]+)"$`, noop)
}

func noop(_ *State, _ []string) error { return nil }
