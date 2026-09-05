package steps

import (
	"fmt"
	"strconv"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// promptChoiceTestID is the dialog's per-choice control: E2E-105 clicks
	// prompt-choice-apply, so a choice's testid is this stem plus its value.
	promptChoiceTestID = "prompt-choice-"
	// promptAnswerInputTestID and promptAnswerSubmitTestID are the typed half of
	// the same dialog, as E2E-106 names them.
	promptAnswerInputTestID  = "prompt-answer-input"
	promptAnswerSubmitTestID = "prompt-answer-submit"
	// applyChoice is the choice an apply loop takes.
	applyChoice = "apply"
	// refinementFeedback is the two lines the refine clause types. ONE newline and
	// no trailing one: a trailing blank line is what ends a multiline read early,
	// which is the fault the clause naming it exists to catch.
	refinementFeedback = "state it as a rule rather than as steps\n" +
		"keep it under 150 words"
)

// registerPromptChoiceSteps binds the reader's own path through a prompt: taking
// a choice, typing an answer, and running a fix loop to its cap.
func registerPromptChoiceSteps(suite *bddgo.Suite[State]) {
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) chooses "([^"]+)" `+
			`on the prompt of (`+runRefPattern+`)$`,
		choosePromptOption)
	suite.Step(
		`^choosing "([^"]+)" on the prompt of (`+runRefPattern+`) `+
			`ends it with outcome "([^"]*)"$`,
		chooseAndAssertOutcome)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) submits "([^"]*)" `+
			`to the clarify prompt of (`+runRefPattern+`)$`,
		submitClarifyAnswer)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) submits two lines of `+
			`refinement feedback with no trailing blank line$`,
		submitRefinementFeedback)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) applies each choice `+
			`prompt of (`+runRefPattern+`), at most (\d+) times$`,
		applyEachChoicePrompt)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) opens the run page `+
			`for the dispatched run$`,
		openDispatchedRunPage)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) checks (`+selectorPattern+`)$`,
		checkControl)
}

// choosePromptOption waits for the run to block on a choice prompt and clicks that
// choice in the dialog the reader is looking at.
func choosePromptOption(state *State, args []string) error {
	label, err := runLabelOf(state, args[2])
	if err != nil {
		return err
	}

	return clickChoice(state, label, args[1])
}

// chooseAndAssertOutcome is the compressed clause: take the choice, then hold the
// run to the ending it produces.
func chooseAndAssertOutcome(state *State, args []string) error {
	label, err := runLabelOf(state, args[1])
	if err != nil {
		return err
	}

	err = clickChoice(state, label, args[0])
	if err != nil {
		return err
	}

	return assertRunOutcome(state, []string{label, args[2]})
}

// clickChoice holds the run to being on a choice prompt, notes it, and clicks the
// control that answers it.
func clickChoice(state *State, label, choice string) error {
	prompt, err := awaitPromptKind(state, label, choiceKind, "")
	if err != nil {
		return err
	}

	notePrompt(state, label, prompt)

	return clickTestID(state, promptChoiceTestID+choice)
}

// clickTestID clicks one control through the same locate-and-click path the
// `clicks` clause takes; its args[0] is the role, which clickElement discards.
func clickTestID(state *State, testID string) error {
	return clickElement(state, []string{"", testID})
}

// submitClarifyAnswer types the number the reader picked into the dialog and
// submits it — the clarify prompt's own control, which a choice prompt has none of.
func submitClarifyAnswer(state *State, args []string) error {
	label, err := runLabelOf(state, args[2])
	if err != nil {
		return err
	}

	prompt, err := awaitPromptKind(state, label, clarifyKind, "")
	if err != nil {
		return err
	}

	notePrompt(state, label, prompt)

	return typeAnswer(state, args[1])
}

// submitRefinementFeedback types the two lines the refine loop takes, on the run
// the scenario last saw a prompt on.
func submitRefinementFeedback(state *State, _ []string) error {
	label := state.Prompted
	if label == "" {
		return state.fail("%w", ErrNoPrompt)
	}

	prompt, err := awaitPromptKind(state, label, freetextKind, "")
	if err != nil {
		return err
	}

	notePrompt(state, label, prompt)

	return typeAnswer(state, refinementFeedback)
}

// typeAnswer fills the dialog's input and submits it.
func typeAnswer(state *State, value string) error {
	_, locator, err := locateStep(state, promptAnswerInputTestID)
	if err != nil {
		return err
	}

	err = locator.Fill(value)
	if err != nil {
		return state.fail("filling %s with %q: %w", promptAnswerInputTestID, value, err)
	}

	return clickTestID(state, promptAnswerSubmitTestID)
}

// applyEachChoicePrompt takes the apply choice on every choice prompt the run
// comes back with, up to the cap the step names, stopping once the run is over.
// The cap is a budget, not a timeout: a loop that never converges fails on it.
func applyEachChoicePrompt(state *State, args []string) error {
	label, err := runLabelOf(state, args[1])
	if err != nil {
		return err
	}

	limit, err := strconv.Atoi(args[2])
	if err != nil {
		return state.fail("the step names %q applies, which is not a number: %w", args[2], err)
	}

	for range limit {
		prompt, over, waitErr := awaitChoiceOrEnd(state, label)
		if waitErr != nil {
			return waitErr
		}

		if over {
			return nil
		}

		notePrompt(state, label, prompt)

		clickErr := clickTestID(state, promptChoiceTestID+applyChoice)
		if clickErr != nil {
			return clickErr
		}

		state.Applies[label]++
	}

	return nil
}

// awaitChoiceOrEnd polls the run until it blocks on a choice prompt the scenario
// has not answered yet, or ends. Both are legal here, which is why this is not
// awaitPromptKind: a run that finished is not a prompt the reader missed.
func awaitChoiceOrEnd(state *State, label string) (*pendingPrompt, bool, error) {
	path, err := runPath(state, label)
	if err != nil {
		return nil, false, err
	}

	deadline := time.Now().Add(runTerminalTimeout)

	var reason string

	for {
		detail, readErr := readRun(state, path)

		switch {
		case readErr != nil:
			reason = readErr.Error()
		case detail.State == stateTerminal:
			return nil, true, nil
		case unansweredChoice(state, label, detail.PendingPrompt):
			return detail.PendingPrompt, false, nil
		default:
			reason = fmt.Sprintf("it is %q", detail.State)
		}

		if !time.Now().Before(deadline) {
			return nil, false, state.fail(
				"run %q neither published a new choice prompt nor ended within %s: %s",
				label, runTerminalTimeout, reason)
		}

		time.Sleep(runPollInterval)
	}
}

// unansweredChoice answers whether the run holds a choice prompt the scenario has
// not already taken: the id the last click answered stays on the run for a beat.
func unansweredChoice(state *State, label string, prompt *pendingPrompt) bool {
	return prompt != nil && prompt.Kind == choiceKind &&
		prompt.PromptID != state.Prompts[label]
}

// openDispatchedRunPage opens the page of the run the page's own action made —
// a disjoint pattern from the labelled `has the run page for run "…" open`.
func openDispatchedRunPage(state *State, args []string) error {
	label, err := runLabelOf(state, dispatchedLabel)
	if err != nil {
		return err
	}

	return openRunPage(state, []string{args[0], label})
}

// checkControl ticks a checkbox rather than clicking it: a click TOGGLES, so a
// row already checked would be cleared by the step that says to check it.
func checkControl(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[1])
	if err != nil {
		return err
	}

	err = locator.Check()
	if err != nil {
		return state.fail("checking %s: %w", sel, err)
	}

	return nil
}
