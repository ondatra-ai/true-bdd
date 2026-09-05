package steps

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// heldToTheWindow is what the document probe answers when the page itself has
// nothing to scroll.
const heldToTheWindow = "held to the window"

// registerPageScrollSteps binds the clauses about what scrolls: a pane a reader
// scrolls to its end, the document that must not scroll with it, and a control
// that still works once it has.
func registerPageScrollSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) scrolls (`+
		selectorPattern+`) to its end$`, scrollToEnd)
	suite.Step(`^the document does not scroll$`, refuteDocumentScrolls)
	suite.Step(`^clicking (`+selectorPattern+`) shows (`+selectorPattern+`)$`,
		assertClickShows)
}

// scrollToEnd scrolls one pane to its own end, which is the state the clauses
// after it read. args[0] is the role, discarded as clickElement's is.
func scrollToEnd(state *State, args []string) error {
	rememberPageState(state)

	sel, locator, err := locateStep(state, args[1])
	if err != nil {
		return err
	}

	_, err = locator.Evaluate(`el => { el.scrollTop = el.scrollHeight }`, nil)
	if err != nil {
		return state.fail("scrolling %s to its end: %w", sel, err)
	}

	return nil
}

// refuteDocumentScrolls holds the document to having nothing to scroll: the
// shell owns the window's height, so a scrollbar there is the one that would
// cover the chat toggle at the right edge.
func refuteDocumentScrolls(state *State, _ []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	read := func() (string, error) { return probeString(page, documentScrollProbe()) }

	got, matched, err := await(read, equals(heldToTheWindow))
	if err != nil {
		return state.fail("reading whether the document scrolls: %w", err)
	}

	if !matched {
		return state.fail("the document %s, want it %s", got, heldToTheWindow)
	}

	return nil
}

// documentScrollProbe answers with the words the clause uses when the document
// has nothing to scroll, and with the numbers behind that judgement when it has.
func documentScrollProbe() string {
	return fmt.Sprintf(`() => {
		const el = document.scrollingElement || document.documentElement
		if (el.scrollHeight <= el.clientHeight + %d) { return %q }
		return "is " + el.scrollHeight + " tall in a window of " + el.clientHeight
	}`, scrollEndSlack, heldToTheWindow)
}

// assertClickShows clicks one control and holds the page to then showing what
// the clause names, so the toggle is graded where the reader left the page.
func assertClickShows(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	err = locator.Click()
	if err != nil {
		return state.fail("clicking %s: %w", sel, err)
	}

	return assertElementShown(state, []string{args[1]})
}
