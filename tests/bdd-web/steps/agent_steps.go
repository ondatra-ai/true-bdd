package steps

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// ErrNoAgent is returned when a clause reads what a register negotiated and no
// earlier step registered an agent.
var ErrNoAgent = errors.New("no earlier step registered an agent")

// ErrRegisterRefused is returned when the relay refused a registration a
// scenario states as a Given.
var ErrRegisterRefused = errors.New("the relay refused the registration")

// overBudgetMargin is how far past the negotiated cap the over-budget clause
// goes: one byte, so the size is the only thing the relay can be refusing.
const overBudgetMargin = 1

// agentSession is one agent this suite registered: the id it chose, the
// correlation the relay negotiated, and the reply cap it was given.
type agentSession struct {
	SessionID       string `json:"-"`
	ConnectionEpoch int    `json:"connection_epoch"`
	ReplyBudget     int    `json:"reply_budget_bytes"`
	CapabilityToken string `json:"capability_token"`
}

// registerAgent is the Given of an agent-protocol scenario: the suite
// registers itself, so a clause about the negotiated budget has one to read
// without a remote in the way.
func registerAgent(state *State, args []string) error {
	label := args[0]

	request, err := buildNamedRequest(state, bodyRequest{
		Method: http.MethodPost,
		Path:   agentRegisterPath,
		Body:   registerBodyName,
		Label:  label,
	})
	if err != nil {
		return err
	}

	response, err := sendRelay(state, request)
	if err != nil {
		return err
	}

	state.Response = response

	if response.Status != http.StatusOK {
		return state.fail("%w: registering agent %q returned %d, want 200: %s",
			ErrRegisterRefused, label, response.Status, response.snippet())
	}

	return rememberAgent(state, label)
}

// rememberAgent keeps what a successful register negotiated, so the poll and
// reply clauses after it correlate rather than guess.
func rememberAgent(state *State, label string) error {
	if state.Response == nil || state.Response.Status != http.StatusOK {
		return nil
	}

	agent := &agentSession{SessionID: agentSessionID(state, label)}

	err := json.Unmarshal(state.Response.Body, agent)
	if err != nil {
		return state.fail("decode the register of agent %q: %w\n%s",
			label, err, state.Response.snippet())
	}

	state.Agent = agent
	state.Agents[label] = agent

	return nil
}

// assertReplyBudget holds the register response to naming a cap: a client that
// is never told its budget cannot respect it.
func assertReplyBudget(state *State, _ []string) error {
	if state.Agent == nil {
		return state.fail("%w", ErrNoAgent)
	}

	if state.Agent.ReplyBudget <= 0 {
		return state.fail(
			"the register response names reply_budget_bytes %d, want a positive number",
			state.Agent.ReplyBudget)
	}

	return nil
}

// assertOverBudgetReply sends a reply one byte past the negotiated cap. The
// correlation travels in headers, so the refusal comes back correlated rather
// than as a blind timeout.
func assertOverBudgetReply(state *State, args []string) error {
	if state.Agent == nil {
		return state.fail("%w", ErrNoAgent)
	}

	path, err := requestPath(state, args[0])
	if err != nil {
		return err
	}

	want, err := parseStatusSet(args[1])
	if err != nil {
		return state.fail("%w", err)
	}

	body, err := oversizeReply(state, state.Agent.ReplyBudget)
	if err != nil {
		return err
	}

	return checkStatus(state, statusClause{
		Request: relayRequest{
			Method:  http.MethodPost,
			Path:    path,
			Body:    body,
			Headers: correlationHeaders(state.Agent),
		},
		Want: want,
	})
}

// oversizeReply pads a well-formed envelope to exactly one byte over the
// budget.
func oversizeReply(state *State, budget int) ([]byte, error) {
	envelope := map[string]any{
		statusKey: http.StatusOK,
		bodyKey:   map[string]any{"pad": ""},
	}

	encoded, err := json.Marshal(envelope)
	if err != nil {
		return nil, state.fail("encode the reply envelope: %w", err)
	}

	padding := budget + overBudgetMargin - len(encoded)
	if padding < overBudgetMargin {
		padding = overBudgetMargin
	}

	envelope[bodyKey] = map[string]any{"pad": strings.Repeat("x", padding)}

	padded, err := json.Marshal(envelope)
	if err != nil {
		return nil, state.fail("encode the padded reply envelope: %w", err)
	}

	return padded, nil
}
