//go:build bdd

// Package steps binds the greeter's Scenario steps to executable Go.
// Nothing compiles it here: the sentinel go.mod fences the tree off, and
// `scen check` only parses it to learn which patterns are registered and
// which concept each definition records.
package steps

// StepFunc runs one step against the scenario state.
type StepFunc func(state *State, args []string) error

// Suite is what the scenario runner offers this package.
type Suite interface {
	Step(pattern string, fn StepFunc)
}

// State is the per-scenario state every definition in this package shares.
type State struct{}

// Register installs one definition, for E2E-002's last step, which
// records no concept: the step-definition check fails that Scenario.
func Register(suite Suite) {
	suite.Step(`^the greeter prints "([^"]+)" on one line$`, printsOneLine)
}

func printsOneLine(_ *State, _ []string) error { return nil }
