package steps

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoPrompt is returned when a clause is about "the same prompt" and no
// earlier step saw a run blocked on one.
var ErrNoPrompt = errors.New("no earlier step saw a run blocked on a prompt")

// ErrNoAnswer is returned when a clause reads what an answer returned and no
// earlier step sent one.
var ErrNoAnswer = errors.New("no earlier step answered a prompt")

// answerRoute hangs below a run's own path: the route the browser posts an
// answer to.
const answerRoute = "/answer"

// answerBody is what that route takes — which prompt is being answered, and the
// value the reader gave it.
type answerBody struct {
	PromptID string `json:"prompt_id"`
	Value    string `json:"value"`
}

// registerPromptAnswerSteps binds the prompt-answer vocabulary: the answer a
// reader sends, the retry a lost response produces, and where the run stands
// afterwards.
func registerPromptAnswerSteps(suite *bddgo.Suite[State]) {
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) answers the pending `+
			`prompt of run "([^"]+)" with "([^"]*)"$`,
		answerPendingPrompt)
	suite.Step(`^answering the same prompt with "([^"]*)" returns status (\d+)$`,
		assertReanswerStatus)
	suite.Step(`^the answer returned status (\d+)$`, assertAnswerStatus)
	suite.Step(`^run "([^"]+)" advances past the prompt$`, assertRunAdvancedPastPrompt)
}

// answerPendingPrompt waits for the run to block on a prompt and answers THAT
// prompt. The status is recorded rather than graded — the scenario's own
// clauses say what the relay owed. args[0] is the role, discarded as openPath's.
func answerPendingPrompt(state *State, args []string) error {
	label, value := args[1], args[2]

	prompt, err := awaitPendingPrompt(state, label)
	if err != nil {
		return err
	}

	rememberPrompt(state, label, prompt.PromptID)
	noteCommandChild(state, label, state.ChildAtAnswer)

	return sendAnswer(state, label, prompt.PromptID, value)
}

// rememberPrompt files the prompt a run was seen blocked on, so a later clause
// about "the same prompt" names that one rather than re-reading a run that may
// have moved on.
func rememberPrompt(state *State, label, promptID string) {
	state.Prompts[label] = promptID
	state.Prompted = label
}

// awaitPendingPrompt waits for the labelled run to block on a prompt and hands
// back the one it is holding: the run is spawned after the dispatch was
// answered, so the first read is a read before it published anything.
func awaitPendingPrompt(state *State, label string) (*pendingPrompt, error) {
	path, err := runPath(state, label)
	if err != nil {
		return nil, err
	}

	deadline := time.Now().Add(promptTimeout)

	var reason string

	for {
		detail, readErr := readRun(state, path)

		switch {
		case readErr != nil:
			reason = readErr.Error()
		case detail.PendingPrompt != nil:
			notePromptEvents(state, label, detail)

			return detail.PendingPrompt, nil
		default:
			reason = fmt.Sprintf("the run is %q and holds no pending prompt", detail.State)
		}

		if !time.Now().Before(deadline) {
			return nil, state.fail("run %q never blocked on a prompt within %s: %s",
				label, promptTimeout, reason)
		}

		time.Sleep(runPollInterval)
	}
}

// sendAnswer posts one answer for a run's prompt and keeps what the relay said,
// so the status clause after it grades THAT response rather than whatever the
// next read replaced it with.
func sendAnswer(state *State, label, promptID, value string) error {
	path, err := runPath(state, label)
	if err != nil {
		return err
	}

	response, err := apiPostJSON(state.RelayURL, path+answerRoute,
		answerBody{PromptID: promptID, Value: value})
	if err != nil {
		return state.fail("answering prompt %q of run %q: %w", promptID, label, err)
	}

	state.Response = response
	state.Answer = response

	return nil
}

// assertReanswerStatus re-sends an answer for the prompt an earlier step saw the
// run blocked on — the retry a lost response produces — and holds the relay to
// the status the step names.
func assertReanswerStatus(state *State, args []string) error {
	value := args[0]

	want, err := strconv.Atoi(args[1])
	if err != nil {
		return state.fail("the step names status %q, which is not a number: %w",
			args[1], err)
	}

	label, promptID, err := lastPrompt(state)
	if err != nil {
		return err
	}

	err = sendAnswer(state, label, promptID, value)
	if err != nil {
		return err
	}

	if state.Answer.Status != want {
		return state.fail("answering prompt %q of run %q with %q returned %d, want %d: %s",
			promptID, label, value, state.Answer.Status, want, state.Answer.snippet())
	}

	return nil
}

// assertAnswerStatus reads the status the answer an earlier step sent came back
// with — the clause a scenario writes when the refusal IS the outcome.
func assertAnswerStatus(state *State, args []string) error {
	want, err := strconv.Atoi(args[0])
	if err != nil {
		return state.fail("the step names status %q, which is not a number: %w",
			args[0], err)
	}

	if state.Answer == nil {
		return state.fail("%w", ErrNoAnswer)
	}

	if state.Answer.Status != want {
		return state.fail("the answer returned %d, want %d: %s",
			state.Answer.Status, want, state.Answer.snippet())
	}

	return nil
}

// assertRunAdvancedPastPrompt holds the run to having LEFT the prompt an earlier
// step saw it on: an answer accepted and then never consumed leaves the run
// sitting on the same prompt id.
func assertRunAdvancedPastPrompt(state *State, args []string) error {
	label := args[0]

	promptID, seen := state.Prompts[label]
	if !seen {
		return state.fail("%w: run %q", ErrNoPrompt, label)
	}

	path, err := runPath(state, label)
	if err != nil {
		return err
	}

	deadline := time.Now().Add(promptTimeout)

	var reason string

	for {
		detail, readErr := readRun(state, path)

		switch {
		case readErr != nil:
			reason = readErr.Error()
		case detail.PendingPrompt == nil || detail.PendingPrompt.PromptID != promptID:
			return nil
		default:
			reason = fmt.Sprintf("it is %q and still holds prompt %q", detail.State, promptID)
		}

		if !time.Now().Before(deadline) {
			return state.fail("run %q never advanced past prompt %q within %s: %s",
				label, promptID, promptTimeout, reason)
		}

		time.Sleep(runPollInterval)
	}
}

// lastPrompt is what a clause means by "the same prompt": the prompt the
// scenario last saw a run blocked on, and the run it belongs to.
func lastPrompt(state *State) (string, string, error) {
	label := state.Prompted
	if label == "" {
		return "", "", state.fail("%w", ErrNoPrompt)
	}

	promptID, seen := state.Prompts[label]
	if !seen {
		return "", "", state.fail("%w: run %q", ErrNoPrompt, label)
	}

	return label, promptID, nil
}
