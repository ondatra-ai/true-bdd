package steps

import (
	"errors"
	"strings"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/tests/libraries/bddgo"
)

// ErrRelayNotRunning is returned when a scenario asserts the relay is up
// and the harness never got it answering.
var ErrRelayNotRunning = errors.New("the relay is not running")

// ErrNavigation is returned when a page did not load.
var ErrNavigation = errors.New("navigation failed")

// textTimeout caps how long a Then step waits for text to appear. The
// page is already loaded by the time one runs, so this is the budget for
// hydration and a client render, not for the network.
const textTimeout = 10_000 // milliseconds

// Register binds every step this suite's scenarios can use — the same
// shape as the CLI suite's Register, deliberately a different vocabulary
// ("opens", "shows", "the page title is") so a step names its surface.
func Register(suite *bddgo.Suite[State]) {
	suite.Step(`^the relay is running$`, relayIsRunning)
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) opens "([^"]*)"$`, openPath)
	suite.Step(`^the page title is "([^"]*)"$`, assertTitle)
	suite.Step(`^the page shows "([^"]*)"$`, assertText)
}

// relayIsRunning asserts the precondition the harness already
// established, so a scenario states it rather than assuming it.
func relayIsRunning(state *State, _ []string) error {
	if state.Harness.BaseURL == "" {
		return state.fail("%w", ErrRelayNotRunning)
	}

	return nil
}

// openPath navigates the scenario's page to a path on the relay. The
// captured role is discarded — held to the product document's role list
// by the pattern itself.
func openPath(state *State, args []string) error {
	path := args[1]

	page, err := state.Context.NewPage()
	if err != nil {
		return state.fail("open a page: %w", err)
	}

	state.Page = page

	_, err = page.Goto(state.Harness.BaseURL+path,
		playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded})
	if err != nil {
		return state.fail("%w: %s%s: %w", ErrNavigation, state.Harness.BaseURL, path, err)
	}

	return nil
}

func assertTitle(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	want := args[0]

	got, err := page.Title()
	if err != nil {
		return state.fail("read the page title: %w", err)
	}

	if got != want {
		return state.fail("page title is %q, want %q", got, want)
	}

	return nil
}

// assertText waits for the text to appear, rather than reading once: the
// page is server-rendered but hydrates client-side, and asserting at
// DOMContentLoaded is how a browser suite acquires the flake it blames on the browser.
func assertText(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	want := args[0]

	locator := page.GetByText(want)

	err = locator.First().WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(textTimeout),
	})
	if err != nil {
		return state.fail("the page never showed %q: %w\n%s", want, err, visibleText(page))
	}

	return nil
}

// visibleText renders what the page DID show, so a missing-text failure
// carries the alternative instead of sending the reader to a screenshot.
func visibleText(page playwright.Page) string {
	body, err := page.Locator("body").InnerText()
	if err != nil {
		return "(the page's text could not be read)"
	}

	return "--- page text ---\n" + strings.TrimSpace(body)
}
