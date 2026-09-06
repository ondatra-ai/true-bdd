package steps

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoStreamedOutput is returned when a clause is about output growing and
// no earlier step read what it grew from.
var ErrNoStreamedOutput = errors.New("no earlier step watched the run's output")

// defaultPromptAnswer is what a clause that must answer a prompt without
// naming a value sends: the first choice, legal for every kind.
const defaultPromptAnswer = "1"

// registerRunStreamSteps binds the run clauses a two-instance scenario needs:
// a dispatch through the instance the step names, the fix flag it carries, the
// output it publishes while still blocked, and the recovery of a committed run.
func registerRunStreamSteps(suite *bddgo.Suite[State]) {
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) dispatches "([^"]+)" `+
			`on the session through relay "([^"]+)" as run "([^"]+)"$`,
		dispatchRunThroughRelay)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) dispatches "([^"]+)" `+
			`on the session with fix through relay "([^"]+)" as run "([^"]+)"$`,
		dispatchFixRunThroughRelay)
	suite.Step(`^run "([^"]+)" has fix set$`, assertRunFix)
	suite.Step(`^run "([^"]+)" streams output before it is answered$`, assertOutputBeforeAnswer)
	suite.Step(`^run "([^"]+)" streams more output after it is answered$`, assertOutputAfterAnswer)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) dispatches "([^"]+)" `+
			`on the session reusing the token of run "([^"]+)"$`,
		redispatchToken)
	suite.Step(`^the project has exactly (\d+) runs?$`, assertProjectRunCount)
	suite.Step(`^the project's only run is run "([^"]+)"$`, assertProjectOnlyRun)
}

// dispatchToken is the idempotency key a labelled dispatch carries. Deterministic
// rather than random: the relay dedups on it, and a scenario's session is fresh,
// so (scenario, label) is already unique.
func dispatchToken(state *State, label string) string {
	return fmt.Sprintf("e2e-%s-%s", state.Scenario.ID, label)
}

// postDispatchTo sends the dispatch the UI sends to one session through one
// relay and records the run under the label the scenario named it by — the
// request the plain, named-session and cross-instance clauses all make.
func postDispatchTo(state *State, baseURL string, session *sessionSummary,
	command, label string, fix bool,
) error {
	return postDispatchSpec(state, baseURL, session, dispatchBody{
		Command:     command,
		Fix:         fix,
		ClientToken: dispatchToken(state, label),
	}, label)
}

// postDispatchSpec sends one dispatch body and files the run it created under the
// label the scenario named it by — the request the protocol clauses and the fix
// clauses both go through.
func postDispatchSpec(state *State, baseURL string, session *sessionSummary,
	body dispatchBody, label string,
) error {
	path := fmt.Sprintf("%s/%s/runs", sessionsPath, session.SessionID)

	response, err := apiPostJSON(baseURL, path, body)
	if err != nil {
		return state.fail("%w", err)
	}

	state.Response = response

	err = recordRun(state, body.Command, label, response)
	if err != nil {
		return err
	}

	// Kept verbatim: the token-replay clause re-sends THIS body, so what it
	// asserts is the relay's dedup and not a rebuilt body's.
	state.Dispatches[label] = body
	state.RunSessions[label] = session.SessionID

	return nil
}

// dispatchRunThroughRelay dispatches through the instance the step names.
// args[0] is the role, discarded as openPath's is.
func dispatchRunThroughRelay(state *State, args []string) error {
	return dispatchThroughRelay(state, args[1], args[2], args[3], false)
}

// dispatchFixRunThroughRelay is the same dispatch with fix set.
func dispatchFixRunThroughRelay(state *State, args []string) error {
	return dispatchThroughRelay(state, args[1], args[2], args[3], true)
}

// dispatchThroughRelay resolves the session out of the SHARED registry and
// dispatches on the other instance: the process sending it never saw the
// remote register.
func dispatchThroughRelay(state *State, command, relayLabel, runLabel string, fix bool) error {
	target, err := lookupRelay(state, relayLabel)
	if err != nil {
		return err
	}

	session, err := ensureSession(state)
	if err != nil {
		return err
	}

	return postDispatchTo(state, target.BaseURL, session, command, runLabel, fix)
}

// assertRunFix holds the run to reading back with the flag it was dispatched
// under: a fix run that reads as a plain one is a run the UI cannot label.
func assertRunFix(state *State, args []string) error {
	label := args[0]

	path, err := runPath(state, label)
	if err != nil {
		return err
	}

	detail, err := readRun(state, path)
	if err != nil {
		return err
	}

	if !detail.Fix {
		return state.fail("run %q reads back with fix unset, want it set", label)
	}

	return nil
}

// assertOutputBeforeAnswer holds the run to publishing output while it is
// still blocked: a run readable only when it ends is a wait, not a watch. What
// it had is remembered, so the clause after the answer compares against it.
func assertOutputBeforeAnswer(state *State, args []string) error {
	label := args[0]

	_, err := awaitPendingPrompt(state, label)
	if err != nil {
		return err
	}

	grown, err := awaitOutputBeyond(state, label, 0)
	if err != nil {
		return err
	}

	state.Outputs[label] = grown

	return nil
}

// assertOutputAfterAnswer answers the prompt the run is holding and holds it
// to publishing MORE than the clause before it saw: output that stops at the
// prompt is a final blob delivered early.
func assertOutputAfterAnswer(state *State, args []string) error {
	label := args[0]

	seen, watched := state.Outputs[label]
	if !watched {
		return state.fail("%w: run %q", ErrNoStreamedOutput, label)
	}

	prompt, err := awaitPendingPrompt(state, label)
	if err != nil {
		return err
	}

	err = answerPromptOfKind(state, label, prompt.Kind, prompt.PromptID, defaultPromptAnswer)
	if err != nil {
		return err
	}

	grown, err := awaitOutputBeyond(state, label, seen)
	if err != nil {
		return err
	}

	state.Outputs[label] = grown

	return nil
}

// awaitOutputBeyond polls the run until its output is longer than what the
// caller already saw, and answers the length it reached.
func awaitOutputBeyond(state *State, label string, seen int) (int, error) {
	path, err := runPath(state, label)
	if err != nil {
		return 0, err
	}

	deadline := time.Now().Add(promptTimeout)

	var reason string

	for {
		detail, readErr := readRun(state, path)

		switch {
		case readErr != nil:
			reason = readErr.Error()
		case len(detail.Output) > seen:
			return len(detail.Output), nil
		default:
			reason = fmt.Sprintf("it is %q and holds %d bytes of output",
				detail.State, len(detail.Output))
		}

		if !time.Now().Before(deadline) {
			return 0, state.fail("run %q never published more than %d bytes of output "+
				"within %s: %s", label, seen, promptTimeout, reason)
		}

		time.Sleep(runPollInterval)
	}
}

// redispatchToken re-sends a run's dispatch VERBATIM, its token included,
// which is how a browser that gave up recovers the run the CLI committed.
func redispatchToken(state *State, args []string) error {
	command, label := args[1], args[2]

	body, dispatched := state.Dispatches[label]
	if !dispatched {
		return state.fail("no earlier step dispatched run %q; the scenario has %s",
			label, strings.Join(recordedIDs(state), ", "))
	}

	if body.Command != command {
		return state.fail("run %q was dispatched as %q, and the step re-sends %q",
			label, body.Command, command)
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

	return nil
}

// runSummary is the run-listing shape: which runs the project holds.
type runSummary struct {
	RunID string `json:"run_id"`
}

// listProjectRuns reads every run the session's project holds — the read the
// "exactly one run" clause is graded on.
func listProjectRuns(state *State) ([]runSummary, error) {
	session, err := ensureSession(state)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("%s/%s/runs", sessionsPath, session.SessionID)

	response, err := apiGet(state.RelayURL, path)
	if err != nil {
		return nil, state.fail("%w", err)
	}

	if response.Status != http.StatusOK {
		return nil, state.fail("GET %s returned %d, want 200: %s",
			path, response.Status, response.snippet())
	}

	var body struct {
		Runs []runSummary `json:"runs"`
	}

	err = json.Unmarshal(response.Body, &body)
	if err != nil {
		return nil, state.fail("decode GET %s: %w\n%s", path, err, response.snippet())
	}

	return body.Runs, nil
}

// runIDs renders what the project holds, so a count failure names them.
func runIDs(runs []runSummary) string {
	if len(runs) == 0 {
		return noneWord
	}

	ids := make([]string, 0, len(runs))

	for _, run := range runs {
		ids = append(ids, run.RunID)
	}

	return strings.Join(ids, ", ")
}

// assertProjectRunCount holds the project to the number of runs the step
// names: a recovered dispatch that created a second one is exactly the fault.
func assertProjectRunCount(state *State, args []string) error {
	want, err := strconv.Atoi(args[0])
	if err != nil {
		return state.fail("the step names %q runs, which is not a number: %w", args[0], err)
	}

	runs, err := listProjectRuns(state)
	if err != nil {
		return err
	}

	if len(runs) != want {
		return state.fail("the project holds %d run(s) (%s), want %d",
			len(runs), runIDs(runs), want)
	}

	return nil
}

// assertProjectOnlyRun holds that one run to being the one the scenario
// labelled, rather than an equivalent the recovery minted.
func assertProjectOnlyRun(state *State, args []string) error {
	label := args[0]

	runID, err := lookupID(state, label)
	if err != nil {
		return err
	}

	runs, err := listProjectRuns(state)
	if err != nil {
		return err
	}

	if len(runs) != 1 || runs[0].RunID != runID {
		return state.fail("the project's runs are %s, want only run %q, which %q names",
			runIDs(runs), runID, label)
	}

	return nil
}
