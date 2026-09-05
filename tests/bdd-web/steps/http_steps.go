package steps

import (
	"encoding/json"
	"net/http"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// registerRelayRequestSteps binds the HTTP-request vocabulary the protocol
// scenarios write: a named body, the headers a step spells out, and the status
// — or statuses — it will accept.
func registerRelayRequestSteps(suite *bddgo.Suite[State]) {
	// Two or more statuses only: the single-status phrasing already has a
	// definition, and this pattern is disjoint from it by the required " or ".
	suite.Step(
		`^a POST to "([^"]+)" with the dispatch body for command "([^"]*)" `+
			`returns status (\d+(?: or \d+)+)$`,
		assertDispatchStatusAmong)
	suite.Step(
		`^a POST to "([^"]+)" with the (`+bodyNames+`) body returns status (`+statusPattern+`)$`,
		assertBodyStatus)
	suite.Step(
		`^a POST to "([^"]+)" with the (`+bodyNames+`) body and headers "([^"]*)" `+
			`returns status (`+statusPattern+`)$`,
		assertBodyStatusWithHeaders)
	suite.Step(
		`^a POST to "([^"]+)" with the (`+bodyNames+`) body does not return status (\d+)$`,
		refuteBodyStatus)
	suite.Step(
		`^a POST to "([^"]+)" with the (`+bodyNames+`) body and headers "([^"]*)" `+
			`does not return status (\d+)$`,
		refuteBodyStatusWithHeaders)
	suite.Step(
		`^a GET to "([^"]+)" with headers "([^"]*)" returns status (`+statusPattern+`)$`,
		assertGetStatusWithHeaders)
	suite.Step(
		`^a POST to each agent route with headers "([^"]*)" returns status (`+statusPattern+`)$`,
		assertEachAgentRouteStatus)
	suite.Step(
		`^a POST to each agent route with headers "([^"]*)" does not return status (\d+)$`,
		refuteEachAgentRouteStatus)
	// The agent-protocol clauses: a registration this suite makes itself, and
	// what the budget it negotiated is then held to. Implemented in
	// agent_steps.go.
	suite.Step(`^an agent "([^"]+)" is registered$`, registerAgent)
	suite.Step(`^the register response carries a numeric reply budget$`, assertReplyBudget)
	suite.Step(
		`^a POST to "([^"]+)" with a body one byte over the reply budget returns status (\d+)$`,
		assertOverBudgetReply)
}

// statusClause is one request clause: what to send, what it accepts, and
// whether the step is naming the status it must NOT be.
type statusClause struct {
	Request relayRequest
	Want    statusSet
	Negated bool
}

// checkStatus makes the request and reports what the relay answered against
// what the step accepts — where every clause in this file ends.
func checkStatus(state *State, clause statusClause) error {
	response, err := sendRelay(state, clause.Request)
	if err != nil {
		return err
	}

	state.Response = response

	if clause.Want.holds(response.Status) == clause.Negated {
		return state.fail("%s %s%s returned %d, want %s: %s",
			clause.Request.Method, requestTarget(state, clause.Request),
			describeHeaders(clause.Request.Headers), response.Status,
			wanted(clause.Want, clause.Negated), response.snippet())
	}

	return nil
}

// wanted renders what the step accepts, its negation included.
func wanted(want statusSet, negated bool) string {
	if negated {
		return "anything but " + want.String()
	}

	return want.String()
}

// assertDispatchStatusAmong is the dispatch clause of a scenario whose outcome
// is legitimately more than one status: a frozen CLI's dispatch times out, or
// the relay has already dropped the session it was for.
func assertDispatchStatusAmong(state *State, args []string) error {
	path, err := requestPath(state, args[0])
	if err != nil {
		return err
	}

	command := args[1]

	want, err := parseStatusSet(args[2])
	if err != nil {
		return state.fail("%w", err)
	}

	body, err := json.Marshal(dispatchBody{
		Command:     command,
		Fix:         false,
		ClientToken: probeToken(state, command),
	})
	if err != nil {
		return state.fail("encode the dispatch of %q: %w", command, err)
	}

	return checkStatus(state, statusClause{
		Request: relayRequest{
			Method:  http.MethodPost,
			Path:    path,
			Body:    body,
			Headers: []headerPair{{Name: originHeader, Value: state.RelayURL}},
		},
		Want: want,
	})
}

// assertBodyStatus is the named-body clause with no headers spelled out — an
// absent Origin included, which is itself what several scenarios assert.
func assertBodyStatus(state *State, args []string) error {
	return runBodyClause(state, bodyRequest{
		Method: http.MethodPost,
		Path:   args[0],
		Body:   args[1],
		Label:  defaultAgentLabel,
	}, args[2], false)
}

// assertBodyStatusWithHeaders is the same clause under the headers the step
// spelled out.
func assertBodyStatusWithHeaders(state *State, args []string) error {
	return runBodyClause(state, bodyRequest{
		Method:  http.MethodPost,
		Path:    args[0],
		Body:    args[1],
		Headers: args[2],
		Label:   defaultAgentLabel,
	}, args[3], false)
}

// refuteBodyStatus is the negative twin: a scenario states the refusal a route
// must NOT give when it cannot state the one it gives.
func refuteBodyStatus(state *State, args []string) error {
	return runBodyClause(state, bodyRequest{
		Method: http.MethodPost,
		Path:   args[0],
		Body:   args[1],
		Label:  defaultAgentLabel,
	}, args[2], true)
}

// refuteBodyStatusWithHeaders is that twin under spelled-out headers.
func refuteBodyStatusWithHeaders(state *State, args []string) error {
	return runBodyClause(state, bodyRequest{
		Method:  http.MethodPost,
		Path:    args[0],
		Body:    args[1],
		Headers: args[2],
		Label:   defaultAgentLabel,
	}, args[3], true)
}

// runBodyClause resolves the route, builds the named body and holds the relay
// to the step's status. A register that succeeded is remembered, so the poll
// and reply clauses after it correlate rather than guess.
func runBodyClause(state *State, want bodyRequest, rawStatus string, negated bool) error {
	path, err := requestPath(state, want.Path)
	if err != nil {
		return err
	}

	want.Path = path

	request, err := buildNamedRequest(state, want)
	if err != nil {
		return err
	}

	accepted, err := parseStatusSet(rawStatus)
	if err != nil {
		return state.fail("%w", err)
	}

	err = checkStatus(state, statusClause{Request: request, Want: accepted, Negated: negated})
	if err != nil {
		return err
	}

	if want.Body == registerBodyName {
		return rememberAgent(state, want.Label)
	}

	return nil
}

// assertGetStatusWithHeaders reads one endpoint under the headers the step
// named — the clause a host-policy scenario writes.
func assertGetStatusWithHeaders(state *State, args []string) error {
	return checkGetWithHeaders(state, "", args[0], args[1], args[2])
}

// checkGetWithHeaders is that read against one relay: an empty baseURL is the
// scenario's own, and a step naming another relay passes that relay's.
func checkGetWithHeaders(state *State, baseURL, rawPath, rawHeaders, rawStatus string) error {
	path, err := requestPath(state, rawPath)
	if err != nil {
		return err
	}

	headers, err := parseHeaders(state, rawHeaders)
	if err != nil {
		return err
	}

	want, err := parseStatusSet(rawStatus)
	if err != nil {
		return state.fail("%w", err)
	}

	return checkStatus(state, statusClause{
		Request: relayRequest{
			Method:  http.MethodGet,
			Path:    path,
			Headers: headers,
			BaseURL: baseURL,
		},
		Want: want,
	})
}

// assertEachAgentRouteStatus holds all three agent routes to one answer — what
// a scenario writes when the policy is the route's rather than the body's.
func assertEachAgentRouteStatus(state *State, args []string) error {
	return runAgentRoutes(state, args[0], args[1], false)
}

// refuteEachAgentRouteStatus is its negative twin.
func refuteEachAgentRouteStatus(state *State, args []string) error {
	return runAgentRoutes(state, args[0], args[1], true)
}

// runAgentRoutes sends each route its own body and fails on the FIRST route
// that answers differently, naming it.
func runAgentRoutes(state *State, rawHeaders, rawStatus string, negated bool) error {
	want, err := parseStatusSet(rawStatus)
	if err != nil {
		return state.fail("%w", err)
	}

	routes := []bodyRequest{
		{
			Method:  http.MethodPost,
			Path:    agentRegisterPath,
			Body:    registerBodyName,
			Headers: rawHeaders,
			Label:   defaultAgentLabel,
		},
		{
			Method:  http.MethodPost,
			Path:    agentPollPath,
			Body:    pollBodyName,
			Headers: rawHeaders,
			Label:   defaultAgentLabel,
		},
		{
			Method:  http.MethodPost,
			Path:    agentReplyPath,
			Body:    replyBodyName,
			Headers: rawHeaders,
			Label:   defaultAgentLabel,
		},
	}

	for _, route := range routes {
		request, buildErr := buildNamedRequest(state, route)
		if buildErr != nil {
			return buildErr
		}

		checkErr := checkStatus(state,
			statusClause{Request: request, Want: want, Negated: negated})
		if checkErr != nil {
			return checkErr
		}
	}

	return nil
}
