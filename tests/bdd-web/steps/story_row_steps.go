package steps

import (
	"fmt"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// checkedReading and uncheckedReading are the words the clause writes, so
	// the poll and the failure speak one vocabulary.
	checkedReading   = "checked"
	uncheckedReading = "not checked"
	// createActionSelector is the row control that creates the story a row
	// names, keyed by the position-derived id the row carries.
	createActionSelector = "story-row[create-id=%s] > action-create"
	// createdOutcome is the ending a create run converges on.
	createdOutcome = "converged"
)

// registerStoryRowSteps binds the story row's vocabulary: the session page a
// scenario opens as a Given, whether a row's toggle was ticked, and the create
// action taken as setup for a clause about what another session then sees.
func registerStoryRowSteps(suite *bddgo.Suite[State]) {
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) has the session page `+
			`of "([^"]+)" open$`,
		openNamedSessionPage)
	suite.Step(`^(`+selectorPattern+`) was (checked|not checked)$`, assertCheckedState)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) has created story "([^"]+)" `+
			`through its row$`,
		createStoryThroughRow)
}

// assertCheckedState holds a control to the tick state the clause names. Polled
// through await, so a row the page re-renders on its own poll is read once it
// has settled rather than mid-render.
func assertCheckedState(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	want := args[1]

	got, matched, err := await(readChecked(locator), equals(want))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s was %s, want it %s", sel, got, want)
	}

	return nil
}

// readChecked renders the control's tick as the words the clause writes.
func readChecked(locator playwright.Locator) func() (string, error) {
	return func() (string, error) {
		checked, err := locator.IsChecked()
		if err != nil {
			return "", fmt.Errorf("read whether the control is checked: %w", err)
		}

		if checked {
			return checkedReading, nil
		}

		return uncheckedReading, nil
	}
}

// createStoryThroughRow takes the row's Create action and waits for the run it
// dispatched to converge — the Given a clause about what a SECOND session reads
// needs, since a story half-written is not a story another CLI can see.
func createStoryThroughRow(state *State, args []string) error {
	role, storyID := args[0], args[1]

	err := clickElement(state, []string{role, fmt.Sprintf(createActionSelector, storyID)})
	if err != nil {
		return err
	}

	label, err := runLabelOf(state, dispatchedLabel)
	if err != nil {
		return err
	}

	return assertRunOutcome(state, []string{label, createdOutcome})
}
