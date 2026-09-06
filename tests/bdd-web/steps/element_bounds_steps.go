package steps

import (
	"fmt"
	"strings"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// chatCloseAnchor is the control a scenario names in words rather than by
	// testid, and chatCloseCSS where it renders: a close control of its own if
	// the chat has one, otherwise the toggle, which is what shuts an open panel.
	chatCloseAnchor = "the chat's close control"
	chatCloseCSS    = `[data-testid="chat-dock-close"], [data-testid="chat-dock-toggle"]`
	// boundsAnchorPattern is what a bounds clause's subject may be: that
	// control, or an element reference in the grammar every other clause reads.
	boundsAnchorPattern = chatCloseAnchor + `|` + selectorPattern
	// writingModeProperty is where a face's direction is settled, and
	// verticalWord the polarity CSS spells `vertical-rl` and `vertical-lr`.
	writingModeProperty = "writing-mode"
	verticalWord        = "vertical"
	// insideWord is what the viewport probe answers for an element rendering
	// wholly within the window.
	insideWord = "inside"
	// viewportFitFunc answers that word, and the rectangle it measured against
	// the window when the element hangs outside it. One pixel of slack:
	// sub-pixel rounding is not a clipped element.
	viewportFitFunc = `(el => {
		const box = el.getBoundingClientRect()
		if (box.top >= -1 && box.left >= -1 &&
			box.bottom <= window.innerHeight + 1 && box.right <= window.innerWidth + 1) {
			return %[1]q
		}
		return "(" + box.left + ", " + box.top + ") to (" + box.right + ", " +
			box.bottom + ") in a viewport of " + window.innerWidth + " by " +
			window.innerHeight
	})`
)

// registerElementBoundsSteps binds the clauses about where a control is drawn:
// the direction its text runs, the panel it sits inside, and the window it
// must not hang outside of.
func registerElementBoundsSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^(`+boundsAnchorPattern+`)'s writing mode is (vertical|horizontal)$`,
		assertWritingMode)
	suite.Step(`^(`+boundsAnchorPattern+`) sits inside (`+selectorPattern+`)$`,
		assertSitsInside)
	suite.Step(`^(`+selectorPattern+`) lies wholly inside the viewport$`,
		assertInsideViewport)
	registerPixelParitySteps(suite)
}

// assertWritingMode holds a control's text direction to the polarity the
// clause names, in either polarity: one definition serves the vertical tab and
// the horizontal button the open panel puts in its header.
func assertWritingMode(state *State, args []string) error {
	subject, locator, err := locateBoundsSubject(state, args[0])
	if err != nil {
		return err
	}

	polarity := args[1]

	got, matched, err := await(readComputedStyle(locator, writingModeProperty),
		writesAs(polarity))
	if err != nil {
		return state.fail("%s: %w", subject, err)
	}

	if !matched {
		return state.fail("%s has %s = %q, want it %s",
			subject, writingModeProperty, got, polarity)
	}

	return nil
}

// writesAs accepts a computed writing mode on the side the clause names: CSS
// spells the vertical modes `vertical-rl` and `vertical-lr`, and a reader sees
// every other value as horizontal.
func writesAs(polarity string) func(string) bool {
	return func(value string) bool {
		return strings.HasPrefix(value, verticalWord) == (polarity == verticalWord)
	}
}

// assertSitsInside holds one control's box wholly within another element's, so
// "in its header" is a measurement rather than a reading of the markup's
// nesting.
func assertSitsInside(state *State, args []string) error {
	subject, locator, err := locateBoundsSubject(state, args[0])
	if err != nil {
		return err
	}

	box, err := locatorBox(state, subject, locator)
	if err != nil {
		return err
	}

	container, outer, err := currentBox(state, args[1])
	if err != nil {
		return err
	}

	if encloses(outer, box) {
		return nil
	}

	return state.fail("%s covers %s and %s covers %s, want it wholly inside",
		subject, renderBox(box), container, renderBox(outer))
}

// assertInsideViewport holds an element to rendering wholly within the window,
// which is what a line clipped below the fold fails.
func assertInsideViewport(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	read := readProbe(locator, fmt.Sprintf(viewportFitFunc, insideWord))

	got, matched, err := await(read, equals(insideWord))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s covers %s, want it wholly inside", sel, got)
	}

	return nil
}

// locateBoundsSubject resolves what a bounds clause's subject names: the
// control named in words through the CSS this suite holds for it, or an
// element reference through the grammar every other clause reads.
func locateBoundsSubject(state *State, text string) (string, playwright.Locator, error) {
	if text != chatCloseAnchor {
		sel, locator, err := locateStep(state, text)

		return sel.String(), locator, err
	}

	page, err := state.page()
	if err != nil {
		return text, nil, err
	}

	locator := page.Locator(chatCloseCSS).First()

	err = locator.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(selectorTimeout),
	})
	if err != nil {
		return text, nil, state.fail("the page never showed %s: %w\n%s",
			text, err, visibleText(page))
	}

	return text, locator, nil
}

// locatorBox measures a subject this suite resolved itself, which currentBox's
// selector grammar cannot name.
func locatorBox(state *State, subject string, locator playwright.Locator,
) (elementBox, error) {
	box, err := locator.BoundingBox()
	if err != nil {
		return elementBox{}, state.fail("measuring %s: %w", subject, err)
	}

	if box == nil {
		return elementBox{}, state.fail("%s: %w", subject, ErrNoBoundingBox)
	}

	return boxOf(box), nil
}

// encloses answers whether the outer rectangle covers the inner one, past the
// slack that keeps sub-pixel rounding from being an overflow.
func encloses(outer, inner elementBox) bool {
	return inner.X >= outer.X-edgeSlack &&
		inner.Y >= outer.Y-edgeSlack &&
		inner.X+inner.Width <= outer.X+outer.Width+edgeSlack &&
		inner.Y+inner.Height <= outer.Y+outer.Height+edgeSlack
}

// renderBox renders a rectangle for a failure, so the reader is told where
// both boxes actually were.
func renderBox(box elementBox) string {
	return fmt.Sprintf("(%.1f, %.1f) %.1f by %.1f", box.X, box.Y, box.Width, box.Height)
}
