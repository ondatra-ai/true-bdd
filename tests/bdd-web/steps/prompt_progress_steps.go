package steps

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// runPageRoute hangs below the session page's route, mirroring the run's own
	// API path: /sessions/<session>/runs/<run>.
	runPageRoute = "/runs/"
	// promptKinds is the alternation a clause names a prompt by, so a step can
	// only be about a kind the CLI publishes.
	promptKinds = choiceKind + "|" + clarifyKind + "|" + freetextKind
)

// registerPromptProgressSteps binds the prompt-progress vocabulary: the run page
// a dialog scenario opens on, the prompt a run must still be holding, and the
// answers that carry it to the next one.
func registerPromptProgressSteps(suite *bddgo.Suite[State]) {
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) has the run page `+
			`for run "([^"]+)" open$`,
		openRunPage)
	suite.Step(`^run "([^"]+)" still waits on the same prompt$`, assertRunStillOnPrompt)
	suite.Step(
		`^answering the (`+promptKinds+`) prompt with "([^"]*)" advances run "([^"]+)" `+
			`to a "([^"]+)" prompt$`,
		assertAnswerAdvancesToKind)
	suite.Step(
		`^answering the (`+promptKinds+`) prompt with "([^"]*)" clears (`+selectorPattern+`)$`,
		assertAnswerClearsElement)
}

// openRunPage opens the page of one run, which hangs below its session's. The
// captured role is discarded, as openPath's is.
func openRunPage(state *State, args []string) error {
	label := args[1]

	runID, err := lookupID(state, label)
	if err != nil {
		return err
	}

	sessionID, dispatched := state.RunSessions[label]
	if !dispatched {
		return state.fail("the scenario recorded no session for run %q; it has %s",
			label, strings.Join(recordedIDs(state), ", "))
	}

	page, err := state.Context.NewPage()
	if err != nil {
		return state.fail("open a page: %w", err)
	}

	state.Page = page
	url := state.RelayURL + sessionRoute + sessionID + runPageRoute + runID

	_, err = page.Goto(url,
		playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded})
	if err != nil {
		return state.fail("%w: %s: %w", ErrNavigation, url, err)
	}

	return nil
}

// assertRunStillOnPrompt holds the run to the prompt it is on: a When that
// answered one leaves it on the next prompt or on none. With no earlier step
// having seen a prompt, the one it holds when the clause starts is that prompt.
func assertRunStillOnPrompt(state *State, args []string) error {
	label := args[0]

	promptID, seen := state.Prompts[label]
	if !seen {
		prompt, err := awaitPendingPrompt(state, label)
		if err != nil {
			return err
		}

		promptID = prompt.PromptID

		rememberPrompt(state, label, promptID)
	}

	return holdPrompt(state, label, promptID)
}

// holdPrompt watches the run over a window and fails on it leaving the prompt.
// Watched rather than read once: an answer that slipped through lands a beat
// after the keystroke the scenario says must not answer.
func holdPrompt(state *State, label, promptID string) error {
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

		if detail.PendingPrompt == nil {
			return state.fail("run %q is %q and holds no pending prompt, "+
				"want it still waiting on %q", label, detail.State, promptID)
		}

		if detail.PendingPrompt.PromptID != promptID {
			return state.fail("run %q now waits on prompt %q, want the same prompt %q",
				label, detail.PendingPrompt.PromptID, promptID)
		}

		if !time.Now().Before(deadline) {
			return nil
		}

		time.Sleep(runPollInterval)
	}
}

// assertAnswerAdvancesToKind answers the prompt of the kind the step names and
// holds the run to coming back on a DIFFERENT prompt of the kind it wants.
func assertAnswerAdvancesToKind(state *State, args []string) error {
	kind, value, label, want := args[0], args[1], args[2], args[3]

	prompt, err := awaitPromptKind(state, label, kind, "")
	if err != nil {
		return err
	}

	err = answerPromptOfKind(state, label, kind, prompt.PromptID, value)
	if err != nil {
		return err
	}

	next, err := awaitPromptKind(state, label, want, prompt.PromptID)
	if err != nil {
		return err
	}

	rememberPrompt(state, label, next.PromptID)

	return nil
}

// assertAnswerClearsElement answers the prompt of the named kind on the run the
// scenario last saw one on, and holds the dialog the step names to going away.
func assertAnswerClearsElement(state *State, args []string) error {
	kind, value, target := args[0], args[1], args[2]

	label := state.Prompted
	if label == "" {
		return state.fail("%w", ErrNoPrompt)
	}

	prompt, err := awaitPromptKind(state, label, kind, "")
	if err != nil {
		return err
	}

	err = answerPromptOfKind(state, label, kind, prompt.PromptID, value)
	if err != nil {
		return err
	}

	return assertElementNotShown(state, []string{target})
}

// answerPromptOfKind sends one answer and holds the relay to accepting it: a
// refused answer would make every clause after it about the wrong failure.
func answerPromptOfKind(state *State, label, kind, promptID, value string) error {
	rememberPrompt(state, label, promptID)

	err := sendAnswer(state, label, promptID, value)
	if err != nil {
		return err
	}

	if state.Answer.Status != http.StatusOK {
		return state.fail("answering the %s prompt of run %q with %q returned %d, want 200: %s",
			kind, label, value, state.Answer.Status, state.Answer.snippet())
	}

	return nil
}

// awaitPromptKind polls the run until it is blocked on a prompt of the named
// kind other than excludeID, naming what it held instead when it gives up.
func awaitPromptKind(state *State, label, kind, excludeID string) (*pendingPrompt, error) {
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
		case detail.PendingPrompt == nil:
			reason = fmt.Sprintf("the run is %q and holds no pending prompt", detail.State)
		case detail.PendingPrompt.PromptID == excludeID:
			reason = fmt.Sprintf("it still holds prompt %q", excludeID)
		case detail.PendingPrompt.Kind != kind:
			reason = fmt.Sprintf("the run holds a %q prompt", detail.PendingPrompt.Kind)
		default:
			notePromptEvents(state, label, detail)

			return detail.PendingPrompt, nil
		}

		if !time.Now().Before(deadline) {
			return nil, state.fail("run %q never blocked on a %q prompt within %s: %s",
				label, kind, promptTimeout, reason)
		}

		time.Sleep(runPollInterval)
	}
}
