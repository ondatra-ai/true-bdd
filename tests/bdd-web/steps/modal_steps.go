package steps

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// isActiveElement and holdsActiveElement are the two focus questions a
	// clause asks: the element itself, or anything inside it.
	isActiveElement    = "el => el === document.activeElement"
	holdsActiveElement = "el => el.contains(document.activeElement)"
	// modalBackdrop is the surface OUTSIDE the panel, which is its own element
	// rather than "somewhere else on the page".
	modalBackdrop = "story-modal-backdrop"
	// openedSelectedAttribute is how the modal stamps the tab it opened on, and
	// openedSelectedValue what that stamp reads: the selection itself moves with
	// the reader, the stamp does not.
	openedSelectedAttribute = "data-opened-selected"
	openedSelectedValue     = "true"
	// attributeSettle is how long a negative attribute clause watches before it
	// accepts the absence, for the reason the absence clause waits rather than
	// reading once: the page fills its data on its own poll.
	attributeSettle = 2 * time.Second
)

// registerModalSteps binds the vocabulary a dialog scenario is written in: the
// keyboard, where focus sits, and the containment form of a text clause.
func registerModalSteps(suite *bddgo.Suite[State]) {
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) presses "([^"]+)"$`,
		pressKey)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) presses "([^"]+)" (\d+) times$`,
		pressKeyTimes)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) focuses (`+selectorPattern+`)$`,
		focusElement)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) has opened the modal for story "([^"]+)"$`,
		openStoryModal)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) clicks the modal backdrop$`,
		clickBackdrop)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) fills (`+selectorPattern+`) `+
			`with "([^"]*)"$`,
		fillElement)
	suite.Step(`^(`+selectorPattern+`) contains text "([^"]*)"$`, assertElementContainsText)
	suite.Step(`^(`+selectorPattern+`) does not contain text "([^"]*)"$`,
		refuteElementContainsText)
	suite.Step(`^(`+selectorPattern+`) is focused$`, assertFocused)
	suite.Step(`^(`+selectorPattern+`) is not focused$`, assertNotFocused)
	suite.Step(`^focus is inside (`+selectorPattern+`)$`, assertFocusInside)
	suite.Step(`^(`+selectorPattern+`) was selected when the modal opened$`, assertOpenedOn)
	suite.Step(`^(`+selectorPattern+`) does not have attribute "([^"]*)" = "([^"]*)"$`,
		refuteAttribute)
}

// pressKey sends one key to the page. The captured role is discarded, as
// openPath's is — held to the product document's role list by the pattern.
func pressKey(state *State, args []string) error {
	return pressTimes(state, args[1], 1)
}

// pressKeyTimes repeats that key, which is how a scenario walks focus through
// a dialog rather than asserting one hop.
func pressKeyTimes(state *State, args []string) error {
	count, err := strconv.Atoi(args[2])
	if err != nil {
		return state.fail("the step names %q times, which is not a number: %w", args[2], err)
	}

	return pressTimes(state, args[1], count)
}

// pressTimes presses one key count times, naming which press failed.
func pressTimes(state *State, key string, count int) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	for press := range count {
		err = page.Keyboard().Press(key)
		if err != nil {
			return state.fail("pressing %q (press %d of %d): %w", key, press+1, count, err)
		}
	}

	return nil
}

// focusElement gives one control focus, which is where an arrow-key clause
// starts from.
func focusElement(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[1])
	if err != nil {
		return err
	}

	err = locator.Focus()
	if err != nil {
		return state.fail("focusing %s: %w", sel, err)
	}

	return nil
}

// openStoryModal is the Given form of the click that opens a story's modal, so
// a scenario about what the modal DOES states opening it as a precondition
// rather than spending its When on it.
func openStoryModal(state *State, args []string) error {
	storyID := args[1]

	err := clickElement(state, []string{args[0],
		fmt.Sprintf("story-row[create-id=%s] > story-title", storyID)})
	if err != nil {
		return err
	}

	return assertElementShown(state,
		[]string{fmt.Sprintf("story-modal[story-id=%s]", storyID)})
}

// clickBackdrop clicks the modal's backdrop, the click a dialog must close on.
func clickBackdrop(state *State, args []string) error {
	return clickElement(state, []string{args[0], modalBackdrop})
}

// fillElement types into one field, replacing what it held — an empty value
// included, which is itself what a rejected-answer scenario submits. The
// captured role is discarded, as clickElement's is.
func fillElement(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[1])
	if err != nil {
		return err
	}

	value := args[2]

	err = locator.Fill(value)
	if err != nil {
		return state.fail("filling %s with %q: %w", sel, value, err)
	}

	return nil
}

// assertElementContainsText holds an element's rendered text to CARRYING the
// step's text — the clause a cell that renders a value beside a label is
// written with, where equality would be about the label.
func assertElementContainsText(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	want := args[1]

	got, matched, err := await(readInnerText(locator),
		func(value string) bool { return strings.Contains(value, want) })
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s reads %q, which does not contain %q", sel, got, want)
	}

	return nil
}

// refuteElementContainsText is the negative twin, watched rather than read
// once: text that arrives a render late would pass a single read.
func refuteElementContainsText(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	unwanted := args[1]
	read := readInnerText(locator)
	deadline := time.Now().Add(attributeSettle)

	for {
		got, readErr := read()
		if readErr != nil {
			return state.fail("%s: %w", sel, readErr)
		}

		if strings.Contains(got, unwanted) {
			return state.fail("%s reads %q, which contains %q, want text that does not",
				sel, got, unwanted)
		}

		if !time.Now().Before(deadline) {
			return nil
		}

		time.Sleep(valuePollInterval)
	}
}

func assertFocused(state *State, args []string) error {
	return checkFocus(state, args[0], isActiveElement, "focused", "focused")
}

func assertNotFocused(state *State, args []string) error {
	return checkFocus(state, args[0], isActiveElement, "not focused", "not focused")
}

func assertFocusInside(state *State, args []string) error {
	return checkFocus(state, args[0], holdsActiveElement, "focused", "focus inside it")
}

// checkFocus polls the focus question and names what it saw: focus lands after
// the render that moved it, so one read is a read mid-transition.
func checkFocus(state *State, text, script, want, clause string) error {
	sel, locator, err := locateStep(state, text)
	if err != nil {
		return err
	}

	got, matched, err := await(readFocus(locator, script), equals(want))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s is %s, want %s", sel, got, clause)
	}

	return nil
}

// readFocus renders the answer to one focus question as the word the step
// uses, so the poll and the failure speak one vocabulary.
func readFocus(locator playwright.Locator, script string) func() (string, error) {
	return func() (string, error) {
		value, err := locator.Evaluate(script, nil)
		if err != nil {
			return "", fmt.Errorf("read whether the element holds focus: %w", err)
		}

		holds, ok := value.(bool)
		if !ok || !holds {
			return "not focused", nil
		}

		return "focused", nil
	}
}

// assertOpenedOn holds a tab to being the one the modal opened on: the
// scenario's own When has already moved to another tab by the time this clause
// runs, so aria-selected would answer a different question.
func assertOpenedOn(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	got, matched, err := await(readAttribute(locator, openedSelectedAttribute),
		equals(openedSelectedValue))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s carries %s = %q, want %q: the modal stamps the tab it "+
			"opened on, and the stamp outlives a later tab change",
			sel, openedSelectedAttribute, got, openedSelectedValue)
	}

	return nil
}

// refuteAttribute is the negative twin of assertAttribute: the element must not
// carry the value the step names. Watched rather than read once, because an
// attribute that arrives a render late would pass a single read.
func refuteAttribute(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	name, unwanted := domAttribute(args[1]), args[2]
	read := readAttribute(locator, name)
	deadline := time.Now().Add(attributeSettle)

	for {
		got, readErr := read()
		if readErr != nil {
			return state.fail("%s: %w", sel, readErr)
		}

		if got == unwanted {
			return state.fail("%s has %s = %q, want anything but that", sel, name, got)
		}

		if !time.Now().Before(deadline) {
			return nil
		}

		time.Sleep(valuePollInterval)
	}
}
