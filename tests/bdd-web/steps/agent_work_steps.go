package steps

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// queueSettle is how long an outstanding call is watched before it counts
	// as queued: one nobody polls must still be outstanding after it.
	queueSettle = 2 * time.Second
	// workPollTimeout is how long an agent's poll is retried before the relay
	// is held to having nothing: work enqueued before a restart is re-served
	// once the process is back, not instantly.
	workPollTimeout = 30 * time.Second
	// redeliveryWindow is how long a delivered item is watched for coming back:
	// exactly-once is a claim about what does NOT arrive.
	redeliveryWindow = 5 * time.Second
)

// pendingRequest is one browser-family call a scenario sent without waiting:
// nothing answers it until an agent polls the work it enqueued.
type pendingRequest struct {
	Session string
	Body    dispatchBody

	done     chan struct{}
	response *apiResponse
	err      error
}

// workItem is one unit of work a poll handed back: the id it is correlated by,
// its type, and the payload the clause after it reads.
type workItem struct {
	WorkID  string          `json:"work_id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`

	// agent is who polled it, so the redelivery clause asks the same one again.
	agent *agentSession
	// relayURL is the instance that handed it over, empty for the scenario's
	// own: a reply goes back to the process holding the caller open.
	relayURL string
}

// registerAgentWorkSteps binds the agent-queue vocabulary: a call left
// outstanding because nothing polls it, the poll that claims it, and what the
// work it hands back must carry.
func registerAgentWorkSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^an agent "([^"]+)" is registered and never polls$`, registerAgent)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) dispatches `+
			`"([^"]+)" on session "([^"]+)" as request "([^"]+)"$`,
		dispatchRequestOnAgent)
	suite.Step(`^the work queue for session "([^"]+)" is non-empty$`, assertQueueNonEmpty)
	suite.Step(`^agent "([^"]+)" polls as work "([^"]+)"$`, agentPollsForWork)
	suite.Step(`^agent "([^"]+)" was not rejected for its original credentials$`,
		assertNotRejected)
	suite.Step(`^work "([^"]+)" has type "([^"]+)"$`, assertWorkType)
	suite.Step(`^work "([^"]+)" carries the dispatch payload of request "([^"]+)"$`,
		assertWorkPayload)
	suite.Step(`^work "([^"]+)" is not redelivered$`, assertWorkNotRedelivered)
}

// dispatchRequestOnAgent sends the dispatch the UI sends to an agent this suite
// registered, WITHOUT waiting: nothing has polled it, so the call stays
// outstanding and the queue it landed in is what the clause after it reads.
func dispatchRequestOnAgent(state *State, args []string) error {
	command, agentLabel, requestLabel := args[1], args[2], args[3]

	agent, err := lookupAgent(state, agentLabel)
	if err != nil {
		return err
	}

	body := dispatchBody{
		Command:     command,
		Fix:         false,
		ClientToken: fmt.Sprintf("e2e-%s-%s", state.Scenario.ID, requestLabel),
	}

	state.Requests[requestLabel] = sendPending(state,
		fmt.Sprintf("%s/%s/runs", sessionsPath, agent.SessionID), agent.SessionID, body)

	return nil
}

// sendPending makes the call on its own goroutine and hands back the record the
// queue and completion clauses read.
func sendPending(state *State, path, session string, body dispatchBody) *pendingRequest {
	return sendPendingTo(state.RelayURL, path, session, body)
}

// assertQueueNonEmpty holds the session's queue to holding work: the calls the
// scenario sent on it are still outstanding, which is what queued means from
// outside — a served one would have answered.
func assertQueueNonEmpty(state *State, args []string) error {
	label := args[0]

	agent, err := lookupAgent(state, label)
	if err != nil {
		return err
	}

	outstanding := 0

	for name, pending := range state.Requests {
		if pending.Session != agent.SessionID {
			continue
		}

		select {
		case <-pending.done:
			return state.fail("request %q on session %q already completed (%s), "+
				"so nothing is queued for it", name, label, describePending(pending))
		case <-time.After(queueSettle):
			outstanding++
		}
	}

	if outstanding == 0 {
		return state.fail("no earlier step sent a call on session %q, so its queue is empty",
			label)
	}

	return nil
}

// describePending renders how a call ended, so a clause naming it says what the
// relay answered rather than only that it answered.
func describePending(pending *pendingRequest) string {
	if pending.err != nil {
		return pending.err.Error()
	}

	if pending.response == nil {
		return "no response"
	}

	return fmt.Sprintf("status %d: %s", pending.response.Status, pending.response.snippet())
}

// agentPollsForWork polls as the agent the scenario registered and files what
// it was handed under the work label. Retried while the relay says 204: an
// empty poll is "nothing yet", not "nothing ever".
func agentPollsForWork(state *State, args []string) error {
	agentLabel, workLabel := args[0], args[1]

	agent, err := lookupAgent(state, agentLabel)
	if err != nil {
		return err
	}

	return pollUntilWork(state, agent, agentLabel, workLabel, "")
}

// pollOnceAt is that poll against one instance: an empty baseURL is the
// scenario's own, and a cross-instance clause names the other process.
func pollOnceAt(state *State, agent *agentSession, baseURL string) (*apiResponse, error) {
	body, err := json.Marshal(agentPollBody(agent))
	if err != nil {
		return nil, state.fail("encode the poll of agent %q: %w", agent.SessionID, err)
	}

	response, err := sendRelay(state, relayRequest{
		Method:  http.MethodPost,
		Path:    agentPollPath,
		Body:    body,
		BaseURL: baseURL,
	})
	if err != nil {
		return nil, err
	}

	state.Response = response

	return response, nil
}

// fileWork decodes the work a poll returned and files it under the label the
// scenario named it by.
func fileWork(state *State, label string, agent *agentSession, response *apiResponse) error {
	var item workItem

	err := json.Unmarshal(response.Body, &item)
	if err != nil {
		return state.fail("decode the work of poll %q: %w\n%s",
			label, err, response.snippet())
	}

	if item.WorkID == "" {
		return state.fail("the poll for work %q named no work_id: %s",
			label, response.snippet())
	}

	item.agent = agent
	state.Works[label] = &item

	return nil
}

// assertNotRejected reads the status the agent's poll actually got: a relay
// that lost its credential store answers a refusal, not work.
func assertNotRejected(state *State, args []string) error {
	label := args[0]

	status, polled := state.Polls[label]
	if !polled {
		return state.fail("no earlier step polled as agent %q", label)
	}

	if status == http.StatusUnauthorized || status == http.StatusForbidden ||
		status == http.StatusConflict {
		return state.fail("agent %q's poll was refused with %d, "+
			"want its original credentials honoured", label, status)
	}

	return nil
}

// assertWorkType holds the delivered item to the kind of work the step names.
func assertWorkType(state *State, args []string) error {
	label, want := args[0], args[1]

	item, err := lookupWork(state, label)
	if err != nil {
		return err
	}

	if item.Type != want {
		return state.fail("work %q has type %q, want %q", label, item.Type, want)
	}

	return nil
}

// assertWorkPayload holds the delivered work to carrying the call's own
// dispatch: the relay hands the CLI what the browser asked for, not a
// re-rendered equivalent.
func assertWorkPayload(state *State, args []string) error {
	workLabel, requestLabel := args[0], args[1]

	item, err := lookupWork(state, workLabel)
	if err != nil {
		return err
	}

	pending, sent := state.Requests[requestLabel]
	if !sent {
		return state.fail("no earlier step sent request %q", requestLabel)
	}

	var payload dispatchBody

	err = json.Unmarshal(item.Payload, &payload)
	if err != nil {
		return state.fail("decode the payload of work %q: %w\n%s",
			workLabel, err, string(item.Payload))
	}

	if payload.Command != pending.Body.Command ||
		payload.ClientToken != pending.Body.ClientToken {
		return state.fail("work %q carries command %q token %q, "+
			"want request %q's command %q token %q",
			workLabel, payload.Command, payload.ClientToken,
			requestLabel, pending.Body.Command, pending.Body.ClientToken)
	}

	return nil
}

// assertWorkNotRedelivered keeps polling and fails on the item coming back:
// delivered-exactly-once is only observable by asking again.
func assertWorkNotRedelivered(state *State, args []string) error {
	label := args[0]

	item, err := lookupWork(state, label)
	if err != nil {
		return err
	}

	deadline := time.Now().Add(redeliveryWindow)

	for time.Now().Before(deadline) {
		response, pollErr := pollOnceAt(state, item.agent, item.relayURL)
		if pollErr != nil {
			return pollErr
		}

		if response.Status == http.StatusOK && carriesWork(response, item.WorkID) {
			return state.fail("work %q was handed out again within %s: %s",
				label, redeliveryWindow, response.snippet())
		}

		time.Sleep(runPollInterval)
	}

	return nil
}

// carriesWork answers whether a poll handed back the same work id.
func carriesWork(response *apiResponse, workID string) bool {
	var item workItem

	err := json.Unmarshal(response.Body, &item)

	return err == nil && item.WorkID == workID
}

// lookupAgent answers which agent a label stands for.
func lookupAgent(state *State, label string) (*agentSession, error) {
	agent, registered := state.Agents[label]
	if !registered {
		return nil, state.fail("%w: %q", ErrNoAgent, label)
	}

	return agent, nil
}

// lookupWork answers which work item a label stands for.
func lookupWork(state *State, label string) (*workItem, error) {
	item, filed := state.Works[label]
	if !filed {
		return nil, state.fail("no earlier step filed work %q", label)
	}

	return item, nil
}
