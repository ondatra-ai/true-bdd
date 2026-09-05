package steps

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// ErrMalformedHeader is returned when a step's headers tail is not "name:
// value" clauses separated by ";".
var ErrMalformedHeader = errors.New("not a \"name: value\" header clause")

// ErrUnknownBody is returned when a step names a request body this suite does
// not build.
var ErrUnknownBody = errors.New("no such request body")

const (
	// missingID is what {MISSING} stands for: a well-formed id the relay can
	// never hold, so its 404 is about absence and not about parsing.
	missingID = "00000000-0000-4000-8000-000000000000"
	// basePlaceholder is what a step writes for the relay's own origin, and
	// missingPlaceholder for an id nothing holds.
	basePlaceholder    = "{BASE}"
	missingPlaceholder = "{MISSING}"
	// hostHeader is applied apart: net/http reads the Host off the request's
	// own field and ignores a header of that name.
	hostHeader = "host"
	// originHeader is the header the relay's browser-family policy reads.
	originHeader = "origin"
	// The three routes "each agent route" stands for.
	agentRegisterPath = "/api/agent/register"
	agentPollPath     = "/api/agent/poll"
	agentReplyPath    = "/api/agent/reply"
	// The bodies a step may name, and the alternation its pattern splices.
	dispatchBodyName = "dispatch"
	registerBodyName = "register"
	pollBodyName     = "poll"
	replyBodyName    = "reply"
	answerBodyName   = "answer"
	bodyNames        = dispatchBodyName + "|" + registerBodyName + "|" +
		pollBodyName + "|" + replyBodyName + "|" + answerBodyName
	// statusPattern is one status, or the statuses a step joins with " or ".
	statusPattern = `\d+(?: or \d+)*`
	// defaultProbeCommand is what a dispatch body carries when the step names
	// no command.
	defaultProbeCommand = "version"
	// defaultAgentLabel files the agent a step registered without naming one.
	defaultAgentLabel = "default"
	// firstEpoch is the connection epoch a poll carries when no register
	// negotiated one.
	firstEpoch = 1
)

// headerPair is one header a step named, kept in the order it named them.
type headerPair struct {
	Name  string
	Value string
}

// relayRequest is one request a clause makes: the route, the body bytes, and
// the headers it is sent under.
type relayRequest struct {
	Method  string
	Path    string
	Body    []byte
	Headers []headerPair
	// BaseURL is the relay this request goes to; empty is the scenario's own,
	// which is every clause that does not name a second relay.
	BaseURL string
}

// bodyRequest is what a step asked for before it is built: the route, the body
// it named, the headers it spelled out, and the agent label they belong to.
type bodyRequest struct {
	Method  string
	Path    string
	Body    string
	Headers string
	Label   string
}

// parseHeaders reads the "name: value; name: value" tail a step writes. Cut on
// the FIRST colon: a value is routinely a URL carrying one of its own.
func parseHeaders(state *State, raw string) ([]headerPair, error) {
	pairs := []headerPair{}

	for _, clause := range strings.Split(raw, ";") {
		trimmed := strings.TrimSpace(clause)
		if trimmed == "" {
			continue
		}

		name, value, found := strings.Cut(trimmed, ":")
		if !found {
			return nil, state.fail("%w: %q in %q", ErrMalformedHeader, trimmed, raw)
		}

		pairs = append(pairs, headerPair{
			Name: strings.ToLower(strings.TrimSpace(name)),
			Value: strings.ReplaceAll(strings.TrimSpace(value),
				basePlaceholder, state.RelayURL),
		})
	}

	return pairs, nil
}

// requestPath resolves the placeholders a step writes into a route: {MISSING}
// first, since no step recorded it, then the ids the scenario did.
func requestPath(state *State, raw string) (string, error) {
	return dispatchPath(state, strings.ReplaceAll(raw, missingPlaceholder, missingID))
}

// buildNamedRequest assembles the request a step described. The step's own
// headers go last, so a clause about a header always overrides.
func buildNamedRequest(state *State, want bodyRequest) (relayRequest, error) {
	body, err := namedBody(state, want.Body, want.Label)
	if err != nil {
		return relayRequest{}, err
	}

	headers := []headerPair{}
	if want.Body == replyBodyName && state.Agent != nil {
		headers = append(headers, correlationHeaders(state.Agent)...)
	}

	spelled, err := parseHeaders(state, want.Headers)
	if err != nil {
		return relayRequest{}, err
	}

	return relayRequest{
		Method:  want.Method,
		Path:    want.Path,
		Body:    body,
		Headers: append(headers, spelled...),
	}, nil
}

// namedBody encodes the body a step named.
func namedBody(state *State, name, label string) ([]byte, error) {
	value, err := bodyValue(state, name, label)
	if err != nil {
		return nil, err
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, state.fail("encode the %s body: %w", name, err)
	}

	return encoded, nil
}

// bodyValue is each body's shape, taken from the remote's own client
// (services/bdd-cli/internal/app/remote/wire.go), so a refusal is about the
// policy under test rather than a body the relay could not read.
func bodyValue(state *State, name, label string) (any, error) {
	switch name {
	case dispatchBodyName:
		return dispatchBody{
			Command:     defaultProbeCommand,
			Fix:         false,
			ClientToken: probeToken(state, defaultProbeCommand),
		}, nil
	case registerBodyName:
		return registerBody(state, label), nil
	case pollBodyName:
		return pollBody(state), nil
	case replyBodyName:
		return map[string]any{statusKey: http.StatusOK, bodyKey: map[string]any{}}, nil
	case answerBodyName:
		return map[string]any{"prompt_id": missingID, "value": "1"}, nil
	default:
		return nil, state.fail("%w: %q", ErrUnknownBody, name)
	}
}

// registerBody is the announcement a remote makes. The session id is the
// scenario's own, so a registration this suite makes is never mistaken for a
// live remote's.
func registerBody(state *State, label string) map[string]any {
	folder := ""
	if state.Tree != nil {
		folder = state.Tree.Dir
	}

	sessionID := agentSessionID(state, label)

	return map[string]any{
		sessionIDField:     sessionID,
		folderKey:          folder,
		"canonical_folder": folder,
		"pid":              os.Getpid(),
		"start_identity":   sessionID,
		"version":          state.Scenario.ID,
	}
}

// pollBody is the correlation triple, from the agent a register step created
// or, with none, an id the relay cannot hold.
func pollBody(state *State) map[string]any {
	if state.Agent == nil {
		return map[string]any{
			sessionIDField:     missingID,
			"connection_epoch": firstEpoch,
			"capability_token": "",
		}
	}

	return agentPollBody(state.Agent)
}

// agentPollBody is one agent's correlation triple — what a poll made on ITS
// behalf carries, whichever label the scenario addressed it by.
func agentPollBody(agent *agentSession) map[string]any {
	return map[string]any{
		sessionIDField:     agent.SessionID,
		"connection_epoch": agent.ConnectionEpoch,
		"capability_token": agent.CapabilityToken,
	}
}

// agentSessionID is the id an agent this suite registers chooses.
func agentSessionID(state *State, label string) string {
	return fmt.Sprintf("e2e-%s-%s", state.Scenario.ID, label)
}

// correlationHeaders is the triple a reply carries OUTSIDE its capped body, so
// an over-cap reply still comes back correlated.
func correlationHeaders(agent *agentSession) []headerPair {
	return []headerPair{
		{Name: "x-session-id", Value: agent.SessionID},
		{Name: "x-connection-epoch", Value: strconv.Itoa(agent.ConnectionEpoch)},
		{Name: "x-capability-token", Value: agent.CapabilityToken},
		{Name: "x-work-id", Value: missingID},
	}
}

// sendRelay makes one request exactly as the step spelled it — no Origin
// unless the step named one, which is what lets a clause assert what an absent
// origin is admitted for.
func sendRelay(state *State, request relayRequest) (*apiResponse, error) {
	var payload io.Reader
	if request.Body != nil {
		payload = bytes.NewReader(request.Body)
	}

	built, err := http.NewRequestWithContext(context.Background(),
		request.Method, relayBase(state, request)+request.Path, payload)
	if err != nil {
		return nil, state.fail("build the %s %s request: %w",
			request.Method, request.Path, err)
	}

	if request.Body != nil {
		built.Header.Set("Content-Type", "application/json")
	}

	applyHeaders(built, request.Headers)

	return readRelay(state, built, request)
}

// applyHeaders sets what the step named, Host through the field net/http reads
// it from rather than the header it ignores.
func applyHeaders(built *http.Request, headers []headerPair) {
	for _, pair := range headers {
		if pair.Name == hostHeader {
			built.Host = pair.Value

			continue
		}

		built.Header.Set(pair.Name, pair.Value)
	}
}

// readRelay sends and reads the whole body: a status clause and the error
// clause after it read one response.
func readRelay(state *State, built *http.Request, request relayRequest) (*apiResponse, error) {
	client := &http.Client{Timeout: apiTimeout}

	response, err := client.Do(built)
	if err != nil {
		return nil, state.fail("%s %s: %w", request.Method, request.Path, err)
	}

	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, state.fail("read the %s %s response: %w",
			request.Method, request.Path, err)
	}

	return &apiResponse{Status: response.StatusCode, Body: body}, nil
}

// describeHeaders renders the headers a request was sent under, so a failure
// names which request it is about.
func describeHeaders(headers []headerPair) string {
	if len(headers) == 0 {
		return ""
	}

	rendered := make([]string, 0, len(headers))

	for _, pair := range headers {
		rendered = append(rendered, pair.Name+": "+pair.Value)
	}

	return fmt.Sprintf(" with headers %q", strings.Join(rendered, "; "))
}

// relayBase is the relay a request goes to: the one the step named, and the
// scenario's own when it named none.
func relayBase(state *State, request relayRequest) string {
	if request.BaseURL != "" {
		return request.BaseURL
	}

	return state.RelayURL
}

// requestTarget is what a failure calls the route: the path alone on the
// scenario's own relay, the whole URL on one a step named, so a two-relay
// failure says which process answered.
func requestTarget(state *State, request relayRequest) string {
	if request.BaseURL == "" || request.BaseURL == state.RelayURL {
		return request.Path
	}

	return request.BaseURL + request.Path
}
