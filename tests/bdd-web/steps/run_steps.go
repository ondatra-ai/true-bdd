package steps

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// promptTimeout is how long a run has to block on a prompt: the dispatch
	// is answered before the child that publishes it has even spawned.
	promptTimeout = 60 * time.Second
	// runPollInterval is how often a run clause re-reads the run.
	runPollInterval = 500 * time.Millisecond
)

// pendingPrompt is the unanswered prompt a run is currently blocked on.
type pendingPrompt struct {
	PromptID string `json:"prompt_id"`
	Kind     string `json:"kind"`
}

// runDetail is the run-scoped read a run clause polls: where the run's state
// machine stands and which prompt it is holding.
type runDetail struct {
	RunID         string         `json:"run_id"`
	State         string         `json:"state"`
	PendingPrompt *pendingPrompt `json:"pending_prompt"`
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
		published, reason := promptState(state, path, kind)
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
func promptState(state *State, path, kind string) (bool, string) {
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

	return true, ""
}

// runPath is the run-scoped read path for the run a scenario labelled.
func runPath(state *State, label string) (string, error) {
	session, err := ensureSession(state)
	if err != nil {
		return "", err
	}

	runID, err := lookupID(state, label)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s/runs/%s", sessionsPath, session.SessionID, runID), nil
}

// readRun reads one run's detail. Deliberately does NOT keep the response:
// this is polled, and the response an error clause reads is a mutation's.
func readRun(state *State, path string) (*runDetail, error) {
	response, err := apiGet(state.Harness.BaseURL, path)
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

	response, err := apiPostJSON(state.Harness.BaseURL,
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
