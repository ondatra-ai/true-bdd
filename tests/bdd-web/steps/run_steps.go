package steps

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// promptTimeout is how long a run has to block on a prompt: the dispatch
	// is answered before the child that publishes it has even spawned.
	promptTimeout = 60 * time.Second
	// runPollInterval is how often a run clause re-reads the run.
	runPollInterval = 500 * time.Millisecond
	// runTerminalTimeout is how long a dispatched run has to finish: the child
	// is a whole CLI invocation, spawned after the dispatch was answered.
	runTerminalTimeout = 180 * time.Second
	// runStayWindow is how long a "not terminal" clause watches: a boot that
	// abandoned somebody else's run ends it within a beat of registering, so
	// reading once is reading before the damage.
	runStayWindow = 5 * time.Second
	// stateTerminal is what the run store calls a run that is over.
	stateTerminal = "terminal"
)

// pendingPrompt is the unanswered prompt a run is currently blocked on.
type pendingPrompt struct {
	PromptID string `json:"prompt_id"`
	Kind     string `json:"kind"`
}

// runEvent is the slice of one run event these clauses read: the server-owned
// sequence number, the type, and the prompt a prompt event names.
type runEvent struct {
	Seq      int    `json:"seq"`
	Type     string `json:"type"`
	PromptID string `json:"prompt_id"`
}

// runDetail is the run-scoped read a run clause polls: where the run's state
// machine stands and which prompt it is holding.
type runDetail struct {
	RunID         string         `json:"run_id"`
	Command       string         `json:"command"`
	StoryID       *string        `json:"story_id"`
	State         string         `json:"state"`
	Outcome       *string        `json:"outcome"`
	PendingPrompt *pendingPrompt `json:"pending_prompt"`
	// Events is the run's whole current capped window, which the clauses about
	// what it recorded after the prompt are graded on.
	Events []runEvent `json:"events"`
	// ErrorDetail is why a terminal run ended badly; Answerable is whether the
	// session this read went through may answer the run's prompt.
	ErrorDetail *string `json:"error_detail"`
	Answerable  bool    `json:"answerable"`
	// Fix is the flag it was dispatched under, and Output what it has published
	// so far — read while the run is open, which is what makes it watchable.
	Fix    bool   `json:"fix"`
	Output string `json:"output"`
}

// registerRunLifecycleSteps binds the clauses about where a dispatched run
// ended up: the state it reaches, and the outcome it carries there.
func registerRunLifecycleSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^run "([^"]+)" reaches a terminal state$`, assertRunTerminal)
	suite.Step(`^run "([^"]+)" has outcome "([^"]*)"$`, assertRunOutcome)
	suite.Step(`^run "([^"]+)" is not terminal$`, assertRunNotTerminal)
	suite.Step(`^run "([^"]+)" has error detail "([^"]*)"$`, assertRunErrorDetail)
}

// assertRunTerminal waits for the labelled run to reach the state the store
// calls terminal — polled, since the run outlives the dispatch that made it.
func assertRunTerminal(state *State, args []string) error {
	_, err := awaitTerminalRun(state, args[0])

	return err
}

// assertRunNotTerminal holds the labelled run to still being open. Watched over
// a window rather than read once, because the clause is about what must NOT
// happen while another remote boots beside it.
func assertRunNotTerminal(state *State, args []string) error {
	label := args[0]

	path, err := runPath(state, label)
	if err != nil {
		return err
	}

	deadline := time.Now().Add(runStayWindow)

	for {
		detail, readErr := readRun(state, path)
		if readErr != nil {
			return readErr
		}

		if detail.State == stateTerminal {
			return state.fail("run %q is %q, want it still open", label, detail.State)
		}

		if !time.Now().Before(deadline) {
			return nil
		}

		time.Sleep(runPollInterval)
	}
}

// assertRunOutcome holds the labelled run's terminal classification to the one
// the step names.
func assertRunOutcome(state *State, args []string) error {
	label, want := args[0], args[1]

	detail, err := awaitTerminalRun(state, label)
	if err != nil {
		return err
	}

	if detail.Outcome == nil {
		return state.fail("run %q is terminal and names no outcome, want %q", label, want)
	}

	if *detail.Outcome != want {
		return state.fail("run %q has outcome %q, want %q", label, *detail.Outcome, want)
	}

	return nil
}

// assertRunErrorDetail holds a terminal run's failure to the reason the step
// names: an outcome alone does not say WHICH failure ended it.
func assertRunErrorDetail(state *State, args []string) error {
	label, want := args[0], args[1]

	detail, err := awaitTerminalRun(state, label)
	if err != nil {
		return err
	}

	if detail.ErrorDetail == nil {
		return state.fail("run %q is terminal and names no error detail, want %q",
			label, want)
	}

	if *detail.ErrorDetail != want {
		return state.fail("run %q has error detail %q, want %q",
			label, *detail.ErrorDetail, want)
	}

	return nil
}

// awaitTerminalRun polls the labelled run until it is terminal and returns the
// reading that said so, so the outcome clause grades ONE snapshot rather than
// re-reading a run that has moved on.
func awaitTerminalRun(state *State, label string) (*runDetail, error) {
	path, err := runPath(state, label)
	if err != nil {
		return nil, err
	}

	deadline := time.Now().Add(runTerminalTimeout)

	var reason string

	for {
		detail, readErr := readRun(state, path)

		switch {
		case readErr != nil:
			reason = readErr.Error()
		case detail.State == stateTerminal:
			return detail, nil
		default:
			reason = fmt.Sprintf("it is %q", detail.State)
		}

		if !time.Now().Before(deadline) {
			return nil, state.fail("run %q never reached a terminal state within %s: %s",
				label, runTerminalTimeout, reason)
		}

		time.Sleep(runPollInterval)
	}
}

// assertPromptPublished waits for the run the scenario labelled to block on
// a prompt of the named kind — polled, since the run is spawned after the
// dispatch was answered.
func assertPromptPublished(state *State, args []string) error {
	label, kind := args[0], args[1]

	path, err := runPath(state, label)
	if err != nil {
		return err
	}

	deadline := time.Now().Add(promptTimeout)

	for {
		published, reason := promptState(state, label, path, kind)
		if published {
			return nil
		}

		if !time.Now().Before(deadline) {
			return state.fail("run %q never published a %q prompt within %s: %s",
				label, kind, promptTimeout, reason)
		}

		time.Sleep(runPollInterval)
	}
}

// promptState answers whether the run is blocked on a prompt of the named
// kind, and what it holds instead when it is not — so the poll's failure
// names what the relay actually said.
func promptState(state *State, label, path, kind string) (bool, string) {
	detail, err := readRun(state, path)
	if err != nil {
		return false, err.Error()
	}

	if detail.PendingPrompt == nil {
		return false, fmt.Sprintf("the run is %q and holds no pending prompt", detail.State)
	}

	if detail.PendingPrompt.Kind != kind {
		return false, fmt.Sprintf("the run holds a %q prompt", detail.PendingPrompt.Kind)
	}

	notePromptEvents(state, label, detail)
	rememberPrompt(state, label, detail.PendingPrompt.PromptID)

	return true, ""
}

// runPath is the run-scoped read path for the run a scenario labelled.
func runPath(state *State, label string) (string, error) {
	runID, err := lookupID(state, label)
	if err != nil {
		return "", err
	}

	// The run's own session, not the scenario's: a scenario may run several
	// remotes, and the dispatch recorded which one accepted this run.
	sessionID, dispatched := state.RunSessions[label]
	if !dispatched {
		return "", state.fail("the scenario recorded no session for run %q; it has %s",
			label, strings.Join(recordedIDs(state), ", "))
	}

	return fmt.Sprintf("%s/%s/runs/%s", sessionsPath, sessionID, runID), nil
}

// readRun reads one run's detail. Deliberately does NOT keep the response:
// this is polled, and the response an error clause reads is a mutation's.
func readRun(state *State, path string) (*runDetail, error) {
	response, err := apiGet(state.RelayURL, path)
	if err != nil {
		return nil, state.fail("%w", err)
	}

	if response.Status != http.StatusOK {
		return nil, state.fail("GET %s returned %d, want 200: %s",
			path, response.Status, response.snippet())
	}

	var detail runDetail

	err = json.Unmarshal(response.Body, &detail)
	if err != nil {
		return nil, state.fail("decode GET %s: %w\n%s", path, err, response.snippet())
	}

	return &detail, nil
}

// assertSameTokenReturnsRun re-sends a labelled dispatch VERBATIM — same
// command, same client token — and holds the relay to answering with the run
// it already created rather than a second one.
func assertSameTokenReturnsRun(state *State, args []string) error {
	label := args[0]

	want, err := strconv.Atoi(args[1])
	if err != nil {
		return state.fail("the step names status %q, which is not a number: %w",
			args[1], err)
	}

	body, dispatched := state.Dispatches[label]
	if !dispatched {
		return state.fail("no earlier step dispatched run %q; the scenario has %s",
			label, strings.Join(recordedIDs(state), ", "))
	}

	session, err := ensureSession(state)
	if err != nil {
		return err
	}

	response, err := apiPostJSON(state.RelayURL,
		fmt.Sprintf("%s/%s/runs", sessionsPath, session.SessionID), body)
	if err != nil {
		return state.fail("%w", err)
	}

	state.Response = response

	if response.Status != want {
		return state.fail("re-dispatching the token of run %q returned %d, want %d: %s",
			label, response.Status, want, response.snippet())
	}

	return assertSameRun(state, label, response)
}

// assertSameRun holds the re-dispatch to naming the run the first dispatch
// created: a 200 carrying a NEW run id would be the second run the scenario
// says an owner can never hold.
func assertSameRun(state *State, label string, response *apiResponse) error {
	var accepted struct {
		RunID string `json:"run_id"`
	}

	err := json.Unmarshal(response.Body, &accepted)
	if err != nil {
		return state.fail("decode the re-dispatch of run %q: %w\n%s",
			label, err, response.snippet())
	}

	if accepted.RunID != state.Runs[label] {
		return state.fail("the re-dispatch returned run %q, want run %q, which run %q names",
			accepted.RunID, state.Runs[label], label)
	}

	return nil
}
