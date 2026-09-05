package steps

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoReregister is returned when a clause is about the epoch a re-register
// retired and no earlier step re-registered that agent.
var ErrNoReregister = errors.New("no earlier step re-registered the agent")

const (
	// wrongCapabilityToken is a token no register ever handed out, so a refusal
	// of it is about the token and not about the session it names.
	wrongCapabilityToken = "not-the-negotiated-capability-token"
	// The two correlation faults a clause may name, and the alternation its
	// pattern splices.
	wrongTokenFault   = "with the wrong capability token"
	supersededFault   = "on the superseded epoch"
	correlationFaults = wrongTokenFault + "|" + supersededFault
	// replyRoute is the word a clause uses for the reply route: a poll carries
	// its correlation in the body and a reply outside it, so the route decides
	// where the faulted triple is written.
	replyRoute = "reply"
	// clientErrorFloor and serverErrorFloor bound what "rejected" means: a 4xx
	// is the relay refusing, anything else is it accepting or breaking.
	clientErrorFloor = 400
	serverErrorFloor = 500
)

// registerAgentEpochSteps binds the correlation vocabulary: the re-register
// that retires an epoch, and what a poll or a reply carrying a retired or
// wrong correlation is owed.
func registerAgentEpochSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^agent "([^"]+)" registers again$`, reregisterAgent)
	suite.Step(`^the new connection epoch is greater than the old one$`, assertEpochAdvanced)
	suite.Step(`^a (poll|reply) for "([^"]+)" (`+correlationFaults+`) is rejected$`,
		assertCorrelationRejected)
}

// reregisterAgent registers the SAME session id a second time, keeping what the
// first registration negotiated: the clauses after it send the correlation this
// register retired.
func reregisterAgent(state *State, args []string) error {
	label := args[0]

	previous, err := lookupAgent(state, label)
	if err != nil {
		return err
	}

	state.PriorAgents[label] = previous
	state.Reregistered = label

	return registerAgent(state, args)
}

// assertEpochAdvanced holds the re-register to minting a LATER epoch: one that
// did not move is one the retired connection could go on answering under.
func assertEpochAdvanced(state *State, _ []string) error {
	label := state.Reregistered
	if label == "" {
		return state.fail("%w", ErrNoReregister)
	}

	previous, renewed := state.PriorAgents[label]
	if !renewed {
		return state.fail("%w: %q", ErrNoReregister, label)
	}

	current, err := lookupAgent(state, label)
	if err != nil {
		return err
	}

	if current.ConnectionEpoch <= previous.ConnectionEpoch {
		return state.fail("agent %q re-registered on connection epoch %d, "+
			"want one greater than the retired %d",
			label, current.ConnectionEpoch, previous.ConnectionEpoch)
	}

	return nil
}

// assertCorrelationRejected makes one agent-route call under a correlation the
// relay must refuse — a token it never issued, or the epoch a re-register
// retired — and holds it to answering a refusal.
func assertCorrelationRejected(state *State, args []string) error {
	route, label, fault := args[0], args[1], args[2]

	agent, err := faultedCorrelation(state, label, fault)
	if err != nil {
		return err
	}

	request, err := correlationRequest(state, route, agent)
	if err != nil {
		return err
	}

	response, err := sendRelay(state, request)
	if err != nil {
		return err
	}

	state.Response = response

	if !isRejection(response.Status) {
		return state.fail("a %s for %q %s returned %d, want a refusal (4xx): %s",
			route, label, fault, response.Status, response.snippet())
	}

	return nil
}

// faultedCorrelation is the triple the clause sends: the retired registration's
// own for a superseded epoch, and the live one under a token no register issued
// for the wrong-token fault.
func faultedCorrelation(state *State, label, fault string) (*agentSession, error) {
	current, err := lookupAgent(state, label)
	if err != nil {
		return nil, err
	}

	if fault == supersededFault {
		previous, renewed := state.PriorAgents[label]
		if !renewed {
			return nil, state.fail("%w: %q", ErrNoReregister, label)
		}

		return previous, nil
	}

	faulted := *current
	faulted.CapabilityToken = wrongCapabilityToken

	return &faulted, nil
}

// correlationRequest is the call the clause makes: a poll writes its triple in
// the body, a reply outside it, which is what lets an over-cap reply still come
// back correlated.
func correlationRequest(state *State, route string,
	agent *agentSession,
) (relayRequest, error) {
	if route == replyRoute {
		body, err := namedBody(state, replyBodyName, defaultAgentLabel)
		if err != nil {
			return relayRequest{}, err
		}

		return relayRequest{
			Method:  http.MethodPost,
			Path:    agentReplyPath,
			Body:    body,
			Headers: correlationHeaders(agent),
		}, nil
	}

	body, err := json.Marshal(agentPollBody(agent))
	if err != nil {
		return relayRequest{}, state.fail("encode the poll of agent %q: %w",
			agent.SessionID, err)
	}

	return relayRequest{Method: http.MethodPost, Path: agentPollPath, Body: body}, nil
}

// isRejection answers whether the relay refused: a 4xx is the refusal these
// clauses are about, and a 2xx or a 5xx is not one.
func isRejection(status int) bool {
	return status >= clientErrorFloor && status < serverErrorFloor
}
