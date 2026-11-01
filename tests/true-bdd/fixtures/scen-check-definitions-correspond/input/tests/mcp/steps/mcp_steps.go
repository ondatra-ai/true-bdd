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

// Register installs one definition per concept a step uses, and none yet
// for the service's log line.
func Register(suite Suite) {
	suite.Step(`^the mcp service is running$`, serviceRunning)
	suite.Step(`^the (Operator|Client Developer) sends "([^"]+)"$`, messageSent)
	suite.Step(`^the service answers "([^"]+)"$`, answer)
}

//truebdd:concept given "service running"
func serviceRunning(_ *State, _ []string) error { return nil }

//truebdd:concept when "message sent"
func messageSent(_ *State, _ []string) error { return nil }

//truebdd:concept then "answer"
func answer(_ *State, _ []string) error { return nil }
