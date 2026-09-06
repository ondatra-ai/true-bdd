package steps

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// requestCompleteTimeout is how long an outstanding call is given to
	// answer: a forwarded one is answered by the relay's own deadline, and past
	// this nothing is going to answer it at all.
	requestCompleteTimeout = 60 * time.Second
	// workIDHeader is where a reply carries which work it answers.
	workIDHeader = "x-work-id"
)

// registerCrossRelayWorkSteps binds the clauses about work that crosses
// instances: a call enqueued on one process, claimed on the other, and what
// each of them then owes the caller.
func registerCrossRelayWorkSteps(suite *bddgo.Suite[State]) {
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) dispatches "([^"]+)" `+
			`on session "([^"]+)" through relay "([^"]+)" as request "([^"]+)"$`,
		dispatchRequestThroughRelay)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) reads the session detail `+
			`of "([^"]+)" on relay "([^"]+)" as request "([^"]+)"$`,
		readSessionDetailOnRelay)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) reads the session detail `+
			`of "([^"]+)" as request "([^"]+)"$`,
		readSessionDetailHere)
	suite.Step(`^agent "([^"]+)" polls relay "([^"]+)" as work "([^"]+)"$`, agentPollsRelay)
	suite.Step(`^agent "([^"]+)" replies to work "([^"]+)"$`, agentRepliesToWork)
	suite.Step(`^request "([^"]+)" completes with status (\d+)$`, assertRequestCompletes)
	suite.Step(`^replying to work "([^"]+)" on relay "([^"]+)" returns status (\d+)$`,
		assertReplyOnRelayStatus)
	// The same call a second time: "again" is what the scenario says about it,
	// not a different request, so both phrasings run one definition.
	suite.Step(`^replying to work "([^"]+)" on relay "([^"]+)" again returns status (\d+)$`,
		assertReplyOnRelayStatus)
	suite.Step(`^replying to work "([^"]+)" returns status (\d+)$`, assertReplyStatus)
	suite.Step(
		`^a reply for "([^"]+)" on relay "([^"]+)" with the wrong capability token `+
			`returns status (`+statusPattern+`)$`,
		assertWrongTokenReplyOnRelay)
	suite.Step(`^(\w+) claimed reads fill the queue for session "([^"]+)"$`, fillSessionQueue)
	suite.Step(
		`^a further status read of "([^"]+)" on relay "([^"]+)" returns status `+
			`(`+statusPattern+`)$`,
		assertFurtherStatusRead)
}

// sendPendingTo makes one mutation against a named relay on its own goroutine.
// The relay is a parameter because a cross-instance scenario sends through the
// instance the step names rather than through the scenario's default.
func sendPendingTo(baseURL, path, session string, body dispatchBody) *pendingRequest {
	pending := &pendingRequest{Session: session, Body: body, done: make(chan struct{})}

	go func() {
		defer close(pending.done)

		pending.response, pending.err = apiPostJSON(baseURL, path, body)
	}()

	return pending
}

// sendPendingRead reads one route without waiting: a read the CLI must serve
// stays outstanding until something polls the work it enqueued.
func sendPendingRead(baseURL, path, session string) *pendingRequest {
	pending := &pendingRequest{Session: session, done: make(chan struct{})}

	go func() {
		defer close(pending.done)

		pending.response, pending.err = apiGet(baseURL, path)
	}()

	return pending
}

// dispatchRequestThroughRelay enqueues a dispatch on the instance the step
// names for a session registered on the other. args[0] is the role, discarded
// as openPath's is.
func dispatchRequestThroughRelay(state *State, args []string) error {
	command, agentLabel, relayLabel, requestLabel := args[1], args[2], args[3], args[4]

	agent, err := lookupAgent(state, agentLabel)
	if err != nil {
		return err
	}

	target, err := lookupRelay(state, relayLabel)
	if err != nil {
		return err
	}

	body := dispatchBody{
		Command:     command,
		Fix:         false,
		ClientToken: fmt.Sprintf("e2e-%s-%s", state.Scenario.ID, requestLabel),
	}

	state.Requests[requestLabel] = sendPendingTo(target.BaseURL,
		fmt.Sprintf("%s/%s/runs", sessionsPath, agent.SessionID), agent.SessionID, body)

	return nil
}

// readSessionDetailOnRelay leaves a detail read outstanding on the instance
// the step names.
func readSessionDetailOnRelay(state *State, args []string) error {
	target, err := lookupRelay(state, args[2])
	if err != nil {
		return err
	}

	return readSessionDetail(state, args[1], target.BaseURL, args[3])
}

// readSessionDetailHere is the same read on the scenario's own relay.
func readSessionDetailHere(state *State, args []string) error {
	return readSessionDetail(state, args[1], state.RelayURL, args[2])
}

// readSessionDetail files the outstanding read under the label the scenario
// named it by, so a later clause grades what finally answered it.
func readSessionDetail(state *State, agentLabel, baseURL, requestLabel string) error {
	agent, err := lookupAgent(state, agentLabel)
	if err != nil {
		return err
	}

	state.Requests[requestLabel] = sendPendingRead(baseURL,
		fmt.Sprintf("%s/%s", sessionsPath, agent.SessionID), agent.SessionID)

	return nil
}

// agentPollsRelay polls the instance the step names for work the OTHER one
// enqueued, and remembers which process handed it over so the reply and the
// redelivery clause go back to the same place.
func agentPollsRelay(state *State, args []string) error {
	agentLabel, relayLabel, workLabel := args[0], args[1], args[2]

	agent, err := lookupAgent(state, agentLabel)
	if err != nil {
		return err
	}

	target, err := lookupRelay(state, relayLabel)
	if err != nil {
		return err
	}

	err = pollUntilWork(state, agent, agentLabel, workLabel, target.BaseURL)
	if err != nil {
		return err
	}

	state.Works[workLabel].relayURL = target.BaseURL

	return nil
}

// pollUntilWork polls one relay until it hands work back, retried while it
// answers 204: an empty poll is "nothing yet", not "nothing ever". An empty
// baseURL is the scenario's own relay.
func pollUntilWork(state *State, agent *agentSession,
	agentLabel, workLabel, baseURL string,
) error {
	deadline := time.Now().Add(workPollTimeout)

	for {
		response, pollErr := pollOnceAt(state, agent, baseURL)
		if pollErr != nil {
			return pollErr
		}

		state.Polls[agentLabel] = response.Status

		if response.Status == http.StatusOK {
			return fileWork(state, workLabel, agent, response)
		}

		if response.Status != http.StatusNoContent || !time.Now().Before(deadline) {
			return state.fail("agent %q polling %s for work %q got %d, want 200 within %s: %s",
				agentLabel, relayNamed(state, baseURL), workLabel, response.Status,
				workPollTimeout, response.snippet())
		}

		time.Sleep(runPollInterval)
	}
}

// relayNamed is which process a failure is about: the one the step named, and
// the scenario's own when it named none.
func relayNamed(state *State, baseURL string) string {
	if baseURL == "" {
		return state.RelayURL
	}

	return baseURL
}

// agentRepliesToWork answers the claimed work on the instance it was polled
// from: the relay forwards that answer to whichever process holds the caller.
func agentRepliesToWork(state *State, args []string) error {
	agentLabel, workLabel := args[0], args[1]

	item, err := lookupWork(state, workLabel)
	if err != nil {
		return err
	}

	body, err := replyPayload(state, workLabel)
	if err != nil {
		return err
	}

	response, err := sendWorkReply(state, item, item.relayURL, body)
	if err != nil {
		return err
	}

	if response.Status != http.StatusOK {
		return state.fail("agent %q replying to work %q on %s returned %d, want 200: %s",
			agentLabel, workLabel, relayNamed(state, item.relayURL),
			response.Status, response.snippet())
	}

	return nil
}

// replyPayload is the envelope an agent answers work with. The marker is
// unique to this scenario and work, so the clause about a reply the relay must
// NOT have kept looks for exactly the bytes this suite sent.
func replyPayload(state *State, workLabel string) ([]byte, error) {
	marker := fmt.Sprintf("e2e-%s-%s-reply", state.Scenario.ID, workLabel)

	encoded, err := json.Marshal(map[string]any{
		statusKey: http.StatusOK,
		bodyKey:   map[string]any{"marker": marker},
	})
	if err != nil {
		return nil, state.fail("encode the reply to work %q: %w", workLabel, err)
	}

	state.LateReplies[workLabel] = marker

	return encoded, nil
}

// sendWorkReply posts one reply for a claimed item. The correlation travels in
// headers, the work id among it, so a reply reaches a caller waiting on
// another instance.
func sendWorkReply(state *State, item *workItem,
	baseURL string, body []byte,
) (*apiResponse, error) {
	response, err := sendRelay(state, relayRequest{
		Method:  http.MethodPost,
		Path:    agentReplyPath,
		Body:    body,
		Headers: workHeaders(item.agent, item.WorkID),
		BaseURL: baseURL,
	})
	if err != nil {
		return nil, err
	}

	state.Response = response

	return response, nil
}

// workHeaders is the shared correlation triple with the work id this reply is
// FOR, which the triple itself leaves standing at an id nothing holds.
func workHeaders(agent *agentSession, workID string) []headerPair {
	headers := correlationHeaders(agent)

	for index := range headers {
		if headers[index].Name == workIDHeader {
			headers[index].Value = workID
		}
	}

	return headers
}

// assertRequestCompletes waits for the call an earlier step left outstanding
// and holds what finally answered it to the step's status.
func assertRequestCompletes(state *State, args []string) error {
	label := args[0]

	want, err := strconv.Atoi(args[1])
	if err != nil {
		return state.fail("the step names status %q, which is not a number: %w", args[1], err)
	}

	pending, sent := state.Requests[label]
	if !sent {
		return state.fail("no earlier step sent request %q", label)
	}

	select {
	case <-pending.done:
	case <-time.After(requestCompleteTimeout):
		return state.fail("request %q had not answered within %s",
			label, requestCompleteTimeout)
	}

	if pending.err != nil {
		return state.fail("request %q never answered: %w", label, pending.err)
	}

	if pending.response.Status != want {
		return state.fail("request %q completed with status %d, want %d: %s",
			label, pending.response.Status, want, pending.response.snippet())
	}

	state.Response = pending.response

	return nil
}

// assertReplyOnRelayStatus replies for the labelled work through the instance
// the step names — a late reply, or the duplicate after it — and holds that
// instance to the answer the scenario states.
func assertReplyOnRelayStatus(state *State, args []string) error {
	return replyAndCheck(state, args[0], args[1], args[2])
}

// assertReplyStatus is the same clause with no instance named: the reply goes
// back to whichever relay handed the work over.
func assertReplyStatus(state *State, args []string) error {
	return replyAndCheck(state, args[0], "", args[1])
}

// replyAndCheck sends one reply and grades the status, naming which process
// answered — a two-instance failure that does not is unreadable.
func replyAndCheck(state *State, workLabel, relayLabel, rawStatus string) error {
	item, err := lookupWork(state, workLabel)
	if err != nil {
		return err
	}

	baseURL := item.relayURL

	if relayLabel != "" {
		target, lookupErr := lookupRelay(state, relayLabel)
		if lookupErr != nil {
			return lookupErr
		}

		baseURL = target.BaseURL
	}

	want, err := strconv.Atoi(rawStatus)
	if err != nil {
		return state.fail("the step names status %q, which is not a number: %w", rawStatus, err)
	}

	body, err := replyPayload(state, workLabel)
	if err != nil {
		return err
	}

	response, err := sendWorkReply(state, item, baseURL, body)
	if err != nil {
		return err
	}

	if response.Status != want {
		return state.fail("replying to work %q on %s returned %d, want %d: %s",
			workLabel, relayNamed(state, baseURL), response.Status, want, response.snippet())
	}

	return nil
}

// assertWrongTokenReplyOnRelay replies for a session registered on the OTHER
// instance under a token no register handed out: the refusal proves that
// instance authenticated out of the shared registry, not out of its own memory.
func assertWrongTokenReplyOnRelay(state *State, args []string) error {
	agentLabel, relayLabel, rawStatus := args[0], args[1], args[2]

	agent, err := lookupAgent(state, agentLabel)
	if err != nil {
		return err
	}

	target, err := lookupRelay(state, relayLabel)
	if err != nil {
		return err
	}

	want, err := parseStatusSet(rawStatus)
	if err != nil {
		return state.fail("%w", err)
	}

	body, err := namedBody(state, replyBodyName, agentLabel)
	if err != nil {
		return err
	}

	faulted := *agent
	faulted.CapabilityToken = wrongCapabilityToken

	return checkStatus(state, statusClause{
		Request: relayRequest{
			Method:  http.MethodPost,
			Path:    agentReplyPath,
			Body:    body,
			Headers: correlationHeaders(&faulted),
			BaseURL: target.BaseURL,
		},
		Want: want,
	})
}

// fillSessionQueue leaves the session's queue holding the number of CLAIMED
// reads the step names: each read is enqueued and then polled, so it occupies
// a slot without being answered — which is what a capacity clause is about.
func fillSessionQueue(state *State, args []string) error {
	count, err := readStepCount(state, args[0])
	if err != nil {
		return err
	}

	label := args[1]

	agent, err := lookupAgent(state, label)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("%s/%s", sessionsPath, agent.SessionID)

	for index := range count {
		name := fmt.Sprintf("%s-queued-%d", label, index+1)

		state.Requests[name] = sendPendingRead(state.RelayURL, path, agent.SessionID)

		err = pollUntilWork(state, agent, label, name, state.RelayURL)
		if err != nil {
			return state.fail("claiming read %d of %d for session %q: %w",
				index+1, count, label, err)
		}
	}

	return nil
}

// readStepCount reads the count a step writes as a word or as a number.
func readStepCount(state *State, raw string) (int, error) {
	count := slices.Index([]string{"", "one", "two", "three", "four", "five"}, raw)
	if count > 0 {
		return count, nil
	}

	count, err := strconv.Atoi(raw)
	if err != nil {
		return 0, state.fail("the step names %q reads, which is neither a number nor "+
			"one, two, three, four or five: %w", raw, err)
	}

	return count, nil
}

// assertFurtherStatusRead reads the session's status view on the instance the
// step names: a queue counted across both processes refuses on either.
func assertFurtherStatusRead(state *State, args []string) error {
	agentLabel, relayLabel, rawStatus := args[0], args[1], args[2]

	agent, err := lookupAgent(state, agentLabel)
	if err != nil {
		return err
	}

	target, err := lookupRelay(state, relayLabel)
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
			Path:    fmt.Sprintf("%s/%s?view=status", sessionsPath, agent.SessionID),
			BaseURL: target.BaseURL,
		},
		Want: want,
	})
}
