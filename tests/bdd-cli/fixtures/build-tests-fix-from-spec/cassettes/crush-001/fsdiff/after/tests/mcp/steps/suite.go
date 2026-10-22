// Package steps holds the step definitions of the `mcp` suite.
//
// Every registry scenario of service mcp-service binds its
// Given/When/Then steps here: Register (in mcp_steps.go) is the single
// call site listing everything this suite binds, and each definition is a
// `suite.Step(`^…$`, fn)` call whose pattern is anchored at both ends.
package steps

import (
	"fmt"
	"regexp"
	"strings"
)

// StepFunc runs one step. args carries the submatches of the definition's
// pattern, in order, so a parameterised definition serves every scenario
// that phrases its step that way.
type StepFunc func(args []string) error

// Definition is one registered pattern and the function it binds.
type Definition struct {
	Pattern *regexp.Regexp
	Fn      StepFunc
}

// Suite is the registry of this suite's step definitions, together with
// the state its definitions share within one scenario.
type Suite struct {
	state *State
	defs  []Definition
}

// NewSuite returns a suite whose definitions all share st.
func NewSuite(st *State) *Suite {
	if st == nil {
		st = NewState()
	}
	return &Suite{state: st}
}

// State is the shared state the definitions of this suite read and write.
func (s *Suite) State() *State { return s.state }

// Step registers fn under an anchored regexp. An unanchored pattern would
// quietly match steps it was never meant to, so Step refuses one.
func (s *Suite) Step(pattern string, fn StepFunc) {
	if len(pattern) < 2 || pattern[0] != '^' || pattern[len(pattern)-1] != '$' {
		panic(fmt.Sprintf("step pattern %q must be anchored with ^ and $", pattern))
	}
	s.defs = append(s.defs, Definition{Pattern: regexp.MustCompile(pattern), Fn: fn})
}

// Definitions returns the registered definitions in registration order.
func (s *Suite) Definitions() []Definition { return s.defs }

// Run executes the one definition whose pattern matches text. A step that
// matches none, or more than one, is not executable, and Run says which it
// was instead of passing.
func (s *Suite) Run(text string) error {
	var matched []Definition
	var args []string
	for _, d := range s.defs {
		if m := d.Pattern.FindStringSubmatch(text); m != nil {
			matched = append(matched, d)
			args = m[1:]
		}
	}
	switch len(matched) {
	case 0:
		return fmt.Errorf("step %q matches no step definition in the mcp suite", text)
	case 1:
		return matched[0].Fn(args)
	default:
		pats := make([]string, 0, len(matched))
		for _, d := range matched {
			pats = append(pats, d.Pattern.String())
		}
		return fmt.Errorf("step %q matches %d step definitions (%s); exactly one must match",
			text, len(matched), strings.Join(pats, ", "))
	}
}
