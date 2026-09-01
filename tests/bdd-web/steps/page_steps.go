package steps

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

const (
	// sessionRoute is the session page's path; the sessions list is "/" and
	// a run view hangs below this.
	sessionRoute = "/sessions/"
	// valueTimeout caps waiting for a rendered value to become the one the
	// step names: the page re-renders on its own poll, so the first read
	// after navigation is not the answer.
	valueTimeout = 15 * time.Second
	// valuePollInterval is how often a value clause re-reads.
	valuePollInterval = 250 * time.Millisecond
)

// openSessionPage navigates to the page for the session the scenario's
// remote registered. The captured role is discarded — held to the product
// document's role list by the pattern itself, as openPath is.
func openSessionPage(state *State, _ []string) error {
	session, err := ensureSession(state)
	if err != nil {
		return err
	}

	page, err := state.Context.NewPage()
	if err != nil {
		return state.fail("open a page: %w", err)
	}

	state.Page = page
	url := state.Harness.BaseURL + sessionRoute + session.SessionID

	_, err = page.Goto(url,
		playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded})
	if err != nil {
		return state.fail("%w: %s: %w", ErrNavigation, url, err)
	}

	return nil
}

// assertElementShown is the presence clause: locate already waits for the
// element to be visible, so the wait IS the assertion.
func assertElementShown(state *State, args []string) error {
	_, _, err := locateStep(state, args[0])

	return err
}

// assertElementNotShown is the absence clause. It WAITS for the element to
// be gone or hidden rather than reading once, because the click that hides
// it re-renders on the client and a single read passes on a stale page.
func assertElementNotShown(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	sel, err := parseSelector(args[0])
	if err != nil {
		return state.fail("%w", err)
	}

	err = sel.element(page).WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateHidden,
		Timeout: playwright.Float(selectorTimeout),
	})
	if err != nil {
		return state.fail("the page still shows %s, want it gone: %w\n%s",
			sel, err, visibleText(page))
	}

	return nil
}

// clickElement is the interaction When: the selector is args[1] because the
// captured role is args[0], discarded — held to the product document's role
// list by the pattern itself, as openPath is.
func clickElement(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[1])
	if err != nil {
		return err
	}

	err = locator.Click()
	if err != nil {
		return state.fail("clicking %s: %w", sel, err)
	}

	return nil
}

// assertAttribute holds one element's attribute to a value.
func assertAttribute(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	name, want := domAttribute(args[1]), args[2]

	got, matched, err := await(readAttribute(locator, name), equals(want))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s has %s = %q, want %q", sel, name, got, want)
	}

	return nil
}

// assertElementText holds one element's rendered text to the step's text.
func assertElementText(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	want := args[1]

	got, matched, err := await(readInnerText(locator), equals(want))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s reads %q, want %q", sel, got, want)
	}

	return nil
}

// assertEnabled holds a control to being operable, polling: a control the
// page has not finished rendering data for may start out disabled.
func assertEnabled(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	got, matched, err := await(readEnabled(locator), equals("enabled"))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s is %s, want enabled", sel, got)
	}

	return nil
}

// assertFieldOfFile holds an element's text to a scalar field of a
// document in the project tree, exactly.
func assertFieldOfFile(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	field, relPath := args[1], args[2]

	want, err := fixtureField(state, relPath, field)
	if err != nil {
		return err
	}

	got, matched, err := await(readInnerText(locator), equals(want))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s reads %q, want the %q of %s, which is %q",
			sel, got, field, relPath, want)
	}

	return nil
}

// assertFieldOfStory holds an element's text to a scalar field of one
// story declared in an epic document. Containment, not equality: the cell
// renders the story's id alongside its title.
func assertFieldOfStory(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	field, storyID, relPath := args[1], args[2], args[3]

	want, err := storyField(state, relPath, storyID, field)
	if err != nil {
		return err
	}

	got, matched, err := await(readInnerText(locator),
		func(value string) bool { return strings.Contains(value, want) })
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s reads %q, which does not carry the %q of story %s of %s, %q",
			sel, got, field, storyID, relPath, want)
	}

	return nil
}

// storyField is the scalar field of one declared story block.
func storyField(state *State, relPath, storyID, field string) (string, error) {
	raw, err := fixtureFile(state, relPath)
	if err != nil {
		return "", err
	}

	block, err := storyBlock(raw, storyID)
	if err != nil {
		return "", state.fail("%s: %w", relPath, err)
	}

	value, err := scalarField(block, field)
	if err != nil {
		return "", state.fail("story %s of %s: %w", storyID, relPath, err)
	}

	return value, nil
}

// locateStep parses the step's selector and resolves it to a visible
// element — the two things every clause above opens with.
func locateStep(state *State, text string) (selector, playwright.Locator, error) {
	sel, err := parseSelector(text)
	if err != nil {
		return selector{}, nil, state.fail("%w", err)
	}

	locator, err := sel.locate(state)
	if err != nil {
		return selector{}, nil, err
	}

	return sel, locator, nil
}

// await re-reads until matches accepts the value, and returns the last
// value it saw with whether it ever matched — so a giving-up caller names
// what the page actually showed rather than only what it wanted.
func await(read func() (string, error), matches func(string) bool) (string, bool, error) {
	deadline := time.Now().Add(valueTimeout)

	for {
		got, err := read()
		if err == nil && matches(got) {
			return got, true, nil
		}

		if !time.Now().Before(deadline) {
			return got, false, err
		}

		time.Sleep(valuePollInterval)
	}
}

func equals(want string) func(string) bool {
	return func(got string) bool { return got == want }
}

// readAttribute reads one attribute. An absent attribute reads as empty,
// which the caller reports against what the step wanted.
func readAttribute(locator playwright.Locator, name string) func() (string, error) {
	return func() (string, error) {
		value, err := locator.GetAttribute(name)
		if err != nil {
			return "", fmt.Errorf("read attribute %s: %w", name, err)
		}

		return value, nil
	}
}

// readInnerText reads an element's rendered text, trimmed: the clause is
// about what a reader sees, not about the markup's whitespace.
func readInnerText(locator playwright.Locator) func() (string, error) {
	return func() (string, error) {
		text, err := locator.InnerText()
		if err != nil {
			return "", fmt.Errorf("read the element's text: %w", err)
		}

		return strings.TrimSpace(text), nil
	}
}

// readEnabled renders the control's state as the word the step uses, so
// the poll and the failure speak one vocabulary.
func readEnabled(locator playwright.Locator) func() (string, error) {
	return func() (string, error) {
		enabled, err := locator.IsEnabled()
		if err != nil {
			return "", fmt.Errorf("read whether the control is enabled: %w", err)
		}

		if enabled {
			return "enabled", nil
		}

		return "disabled", nil
	}
}

// assertElementCount holds how many elements the selector matches to the
// number the step names, polling: the page renders its lists off its own
// poll, so a count read once is a count read mid-render.
func assertElementCount(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	sel, err := parseSelector(args[1])
	if err != nil {
		return state.fail("%w", err)
	}

	want := args[0]

	got, matched, err := await(readCount(page, sel), equals(want))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("the page shows %s %s, want exactly %s", got, sel, want)
	}

	return nil
}

// readCount reads how many elements the selector matches, rendered as the
// digits the step writes so the poll and the failure speak one vocabulary.
// Matched, not visible — the same count the absence clause reasons about.
func readCount(page playwright.Page, sel selector) func() (string, error) {
	return func() (string, error) {
		count, err := sel.element(page).Count()
		if err != nil {
			return "", fmt.Errorf("count the elements matching %s: %w", sel, err)
		}

		return strconv.Itoa(count), nil
	}
}
