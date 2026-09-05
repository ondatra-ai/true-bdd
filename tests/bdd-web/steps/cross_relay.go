package steps

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoLabelledRelay is returned when a clause names a relay no Given started.
var ErrNoLabelledRelay = errors.New("no Given step started a relay under that label")

// registerCrossRelaySteps binds the two-instance vocabulary: the pair of
// relays a scenario starts over one shared backend, and what each of them
// then says about a session or a remote the other one took.
func registerCrossRelaySteps(suite *bddgo.Suite[State]) {
	suite.Step(`^relays "([^"]+)" and "([^"]+)" share one Redis$`, startSharedRelays)
	suite.Step(
		`^relays "([^"]+)" and "([^"]+)" share one Redis with "([^"]+)" set to "([^"]*)"$`,
		startConfiguredSharedRelays)
	suite.Step(`^an agent "([^"]+)" is registered on relay "([^"]+)"$`, registerAgentOnRelay)
	suite.Step(`^session "([^"]+)" is listed on relay "([^"]+)"$`, assertSessionListedOnRelay)
	suite.Step(`^a remote is running against relay "([^"]+)"$`, startRemoteAgainstRelay)
}

// startSharedRelays starts the labelled pair over one backend.
func startSharedRelays(state *State, args []string) error {
	return bootSharedRelays(state, args[0], args[1])
}

// startConfiguredSharedRelays starts that pair with one further setting on
// BOTH: a cap the deployment sets is one both instances must count under.
func startConfiguredSharedRelays(state *State, args []string) error {
	return bootSharedRelays(state, args[0], args[1], args[2]+"="+args[3])
}

// bootSharedRelays proves the backend answers, starts one relay per label
// under this scenario's own key prefix, and points every clause that names no
// relay at the FIRST label — the instance a cross-instance scenario reads through.
func bootSharedRelays(state *State, first, second string, settings ...string) error {
	client, err := dialRedis(state)
	if err != nil {
		return err
	}

	client.close()

	for _, label := range []string{first, second} {
		started, startErr := startScenarioRelay(state,
			append(redisEnv(state), settings...)...)
		if startErr != nil {
			return state.fail("starting relay %q over %s: %w", label, redisURL, startErr)
		}

		state.Relays[label] = started
	}

	state.RelayURL = state.Relays[first].BaseURL
	state.SecondRelayURL = state.Relays[second].BaseURL

	return nil
}

// lookupRelay answers which process a label stands for.
func lookupRelay(state *State, label string) (*relay, error) {
	started, ok := state.Relays[label]
	if !ok {
		return nil, state.fail("%w: %q; the scenario started %s",
			ErrNoLabelledRelay, label, strings.Join(relayLabels(state), ", "))
	}

	return started, nil
}

// relayLabels lists the labels a relay was started under, so a failure names
// what the scenario has rather than only what it wanted.
func relayLabels(state *State) []string {
	if len(state.Relays) == 0 {
		return []string{noneWord}
	}

	labels := make([]string, 0, len(state.Relays))

	for label := range state.Relays {
		labels = append(labels, label)
	}

	sort.Strings(labels)

	return labels
}

// registerAgentOnRelay registers an agent of this suite's own on the instance
// the step names, so the clause after it can ask the OTHER one about it.
func registerAgentOnRelay(state *State, args []string) error {
	label, relayLabel := args[0], args[1]

	target, err := lookupRelay(state, relayLabel)
	if err != nil {
		return err
	}

	request, err := buildNamedRequest(state, bodyRequest{
		Method: http.MethodPost,
		Path:   agentRegisterPath,
		Body:   registerBodyName,
		Label:  label,
	})
	if err != nil {
		return err
	}

	request.BaseURL = target.BaseURL

	response, err := sendRelay(state, request)
	if err != nil {
		return err
	}

	state.Response = response

	if response.Status != http.StatusOK {
		return state.fail("%w: registering agent %q on relay %q returned %d, want 200: %s",
			ErrRegisterRefused, label, relayLabel, response.Status, response.snippet())
	}

	return rememberAgent(state, label)
}

// assertSessionListedOnRelay polls the named instance's registry for a session
// the OTHER one registered: shared rather than held in a process is only
// observable from the process that never saw the register.
func assertSessionListedOnRelay(state *State, args []string) error {
	label, relayLabel := args[0], args[1]

	agent, err := lookupAgent(state, label)
	if err != nil {
		return err
	}

	target, err := lookupRelay(state, relayLabel)
	if err != nil {
		return err
	}

	deadline := time.Now().Add(sessionAppearTimeout)

	var reason string

	for {
		entry, listed, readErr := findListed(target.BaseURL, agent.SessionID)

		switch {
		case readErr != nil:
			reason = readErr.Error()
		case entry != nil:
			state.Sessions[label] = entry

			return nil
		default:
			reason = fmt.Sprintf("it lists %d other session(s)", listed)
		}

		if !time.Now().Before(deadline) {
			return state.fail("relay %q never listed session %q (%s) within %s: %s",
				relayLabel, label, agent.SessionID, sessionAppearTimeout, reason)
		}

		time.Sleep(sessionPollInterval)
	}
}

// findListed reads one relay's registry and picks a session out of it. A read
// that failed is an error and an absent session a nil entry — the appearance
// clause and the sweep clause need those told apart.
func findListed(baseURL, sessionID string) (*sessionSummary, int, error) {
	sessions, err := listSessions(baseURL)
	if err != nil {
		return nil, 0, err
	}

	for index := range sessions {
		if sessions[index].SessionID == sessionID {
			return &sessions[index], len(sessions), nil
		}
	}

	return nil, len(sessions), nil
}

// startRemoteAgainstRelay runs a real remote against the instance the step
// names, while every clause that names none reads the other one.
func startRemoteAgainstRelay(state *State, args []string) error {
	if state.Tree == nil {
		return state.fail("%w", ErrNoProjectTree)
	}

	target, err := lookupRelay(state, args[0])
	if err != nil {
		return err
	}

	remote, err := startRemote(state.Harness, target.BaseURL, state.Tree.Dir)
	if err != nil {
		return state.fail("%w", err)
	}

	state.T.Cleanup(remote.stop)

	state.Remote = remote

	return nil
}
