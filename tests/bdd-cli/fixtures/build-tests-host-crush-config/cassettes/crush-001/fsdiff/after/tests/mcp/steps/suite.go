// Package steps holds the executable step definitions of the `mcp`
// suite: every phrase docs/scenarios.yaml uses for service
// `mcp-service` binds to exactly one anchored pattern registered here.
package steps

import (
	"fmt"
	"regexp"
)

// State is what every step of this suite shares: the server a Given
// resolved, and the exchange a When recorded for the Thens to read.
type State struct {
	BaseURL    string
	Port       int
	RequestURL string
	StatusCode int
	Body       []byte
}

// StepFunc runs one step. args are the capture groups of the pattern
// that matched the step's text, in order; an absent group is "".
type StepFunc func(s *State, args ...string) error

type definition struct {
	pattern *regexp.Regexp
	fn      StepFunc
}

// Suite is the set of definitions Register fills; the runner matches a
// scenario's step text against the patterns in it.
type Suite struct {
	State *State
	defs  []definition
}

// NewSuite returns a suite with every definition of this package
// registered against a fresh State.
func NewSuite() *Suite {
	suite := &Suite{State: &State{}}
	Register(suite)

	return suite
}

// Step registers one definition. The pattern must be anchored: an
// unanchored one quietly matches steps it was never meant to.
func (su *Suite) Step(pattern string, fn StepFunc) {
	su.defs = append(su.defs, definition{pattern: regexp.MustCompile(pattern), fn: fn})
}

// Run executes the single definition whose pattern matches text. Two
// matches is as much a failure as none — neither is a step the suite
// can run.
func (su *Suite) Run(text string) error {
	var (
		found *definition
		args  []string
	)

	for i := range su.defs {
		groups := su.defs[i].pattern.FindStringSubmatch(text)
		if groups == nil {
			continue
		}

		if found != nil {
			return fmt.Errorf(
				"expected step %q to match one definition in the mcp suite, got both %q and %q",
				text, found.pattern.String(), su.defs[i].pattern.String(),
			)
		}

		found = &su.defs[i]
		args = groups[1:]
	}

	if found == nil {
		return fmt.Errorf("expected step %q to match a definition in the mcp suite, got none", text)
	}

	return found.fn(su.State, args...)
}
