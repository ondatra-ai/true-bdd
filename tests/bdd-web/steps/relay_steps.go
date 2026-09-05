package steps

import (
	"errors"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoSecondRelay is returned when a clause is about the second relay and no
// Given step started one.
var ErrNoSecondRelay = errors.New("no Given step started a second relay")

// registerRelayProcessSteps binds the relay-process vocabulary: a relay the
// scenario configures for itself, a second one beside it, and the read that
// names which of the two it is about.
func registerRelayProcessSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the relay is running with "([^"]+)" set to "([^"]*)"$`, startConfiguredRelay)
	suite.Step(`^a second relay is running$`, startSecondRelay)
	suite.Step(
		`^a GET to "([^"]+)" on the second relay with headers "([^"]*)" `+
			`returns status (`+statusPattern+`)$`,
		assertSecondRelayGetStatus)
}

// startConfiguredRelay starts a relay of the scenario's own under the setting
// the step named and points every later clause at it: a mode the deployment
// sets is one the shared harness relay cannot be asked for.
func startConfiguredRelay(state *State, args []string) error {
	name, value := args[0], args[1]

	started, err := startScenarioRelay(state, append(redisEnv(state), name+"="+value)...)
	if err != nil {
		return state.fail("starting a relay with %s set to %q: %w", name, value, err)
	}

	state.RelayURL = started.BaseURL
	state.Relay = started

	return nil
}

// startSecondRelay starts a plain relay beside the scenario's own, so a clause
// can hold a differently-configured process to a different answer.
func startSecondRelay(state *State, _ []string) error {
	started, err := startScenarioRelay(state, redisEnv(state)...)
	if err != nil {
		return state.fail("starting a second relay: %w", err)
	}

	state.SecondRelayURL = started.BaseURL

	return nil
}

// assertSecondRelayGetStatus reads one endpoint on the relay a Given started
// BESIDE the scenario's own, under the headers the step named.
func assertSecondRelayGetStatus(state *State, args []string) error {
	if state.SecondRelayURL == "" {
		return state.fail("%w", ErrNoSecondRelay)
	}

	return checkGetWithHeaders(state, state.SecondRelayURL, args[0], args[1], args[2])
}
