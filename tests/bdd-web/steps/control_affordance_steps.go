package steps

import (
	"fmt"
	"strings"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// colourChannel and backgroundChannel are the two channels a hover may answer
	// on, named as the clause names them.
	colourChannel     = "colour"
	backgroundChannel = "background"
)

// paintReadingFunc reads both channels at once, joined by the NUL every paired
// reading in this suite uses: a hover may answer on either.
const paintReadingFunc = `((el) => {
	const style = getComputedStyle(el)
	return style.color.trim() + "\u0000" + style.backgroundColor.trim()
})`

// filledFunc answers ok when the element paints its own surface — and, when the
// clause allows a border instead, when it draws one — and with what it read when
// it does neither.
const filledFunc = `((el, bordered, ok) => {
	const style = getComputedStyle(el)
	const background = style.backgroundColor.trim()
	if (background !== "transparent" && !/,\s*0\)$/.test(background)) { return ok }
	if (bordered) {
		for (const side of ["top", "right", "bottom", "left"]) {
			const width = parseFloat(style.getPropertyValue("border-" + side + "-width")) || 0
			const drawn = style.getPropertyValue("border-" + side + "-style").trim()
			if (width > 0 && drawn !== "none") { return ok }
		}
	}
	return "background-color: " + background + ", border-top: " +
		style.borderTopWidth + " " + style.borderTopStyle
})`

// placeholderFunc is the prompt a field shows while it is empty, read off an
// inner input when the element the clause names wraps one.
const placeholderFunc = `((el) => {
	const field = el.hasAttribute("placeholder") ? el : el.querySelector("[placeholder]")
	return field ? (field.getAttribute("placeholder") || "").trim() : ""
})`

// registerControlAffordanceSteps binds the clauses about what a control offers a
// reader: the answer it gives the pointer, the fill that marks it pressable, and
// the placeholder saying what to type into it.
func registerControlAffordanceSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^hovering (`+selectorPattern+`) changes its `+
		`(colour or background|colour|background)$`, assertHoverChangesPaint)
	suite.Step(`^(`+selectorPattern+`) is filled( or bordered)?$`, assertFilled)
	suite.Step(`^(`+selectorPattern+`) has a non-empty placeholder$`,
		assertNonEmptyPlaceholder)
}

// assertHoverChangesPaint holds a control to answering the pointer on the channel
// the clause names: a row that answers with nothing reads as dead. Measured on
// the FIRST match — the clause is about a kind of row, not about one of them.
func assertHoverChangesPaint(state *State, args []string) error {
	sel, locator, err := firstOfStep(state, args[0])
	if err != nil {
		return err
	}

	channel := args[1]

	before, err := readHoverPaint(locator)()
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	err = locator.Hover()
	if err != nil {
		return state.fail("hovering %s: %w", sel, err)
	}

	got, matched, err := await(readHoverPaint(locator), paintChanged(before, channel))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s paints %s under the pointer and painted %s before it, "+
			"want the hover to change its %s",
			sel, renderPaint(got), renderPaint(before), channel)
	}

	return nil
}

// assertFilled holds a control to being marked as pressable: a surface of its
// own, or an outline where the clause allows one instead.
func assertFilled(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	allowance := args[1]
	read := readProbe(locator, fmt.Sprintf(`el => %s(el, %t, %q)`,
		filledFunc, allowance != "", verdictOK))

	got, matched, err := await(read, equals(verdictOK))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s renders %s, want it filled%s", sel, got, allowance)
	}

	return nil
}

// assertNonEmptyPlaceholder holds a field to saying what to type into it, which
// an empty placeholder attribute does not.
func assertNonEmptyPlaceholder(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	read := readProbe(locator, `el => `+placeholderFunc+`(el)`)

	got, matched, err := await(read, func(value string) bool { return value != "" })
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s offers the placeholder %q, want one saying what to type",
			sel, got)
	}

	return nil
}

// readHoverPaint reads both channels a hover may answer on, as a reader, so the
// hover clause polls through the same await every other value clause uses.
func readHoverPaint(locator playwright.Locator) func() (string, error) {
	return readProbe(locator, `el => `+paintReadingFunc+`(el)`)
}

// paintChanged accepts a reading whose named channel differs from the one the
// element painted before the pointer arrived.
func paintChanged(before, channel string) func(string) bool {
	wasColour, wasBackground, _ := strings.Cut(before, linkFieldSeparator)

	return func(now string) bool {
		isColour, isBackground, _ := strings.Cut(now, linkFieldSeparator)

		switch channel {
		case colourChannel:
			return isColour != wasColour
		case backgroundChannel:
			return isBackground != wasBackground
		default:
			return isColour != wasColour || isBackground != wasBackground
		}
	}
}

// renderPaint renders a paired paint reading for a failure, so the reader is
// told both channels rather than the NUL between them.
func renderPaint(reading string) string {
	colour, background, _ := strings.Cut(reading, linkFieldSeparator)

	return "color: " + colour + ", background-color: " + background
}

// firstOfStep resolves a step's element reference to its FIRST match: a clause
// about how a kind of row answers the pointer is about any one of them, and the
// strict lookup every other clause uses refuses a page rendering several.
func firstOfStep(state *State, text string) (selector, playwright.Locator, error) {
	sel, err := parseSelector(text)
	if err != nil {
		return selector{}, nil, state.fail("%w", err)
	}

	page, err := state.page()
	if err != nil {
		return sel, nil, err
	}

	locator := sel.element(page).First()

	err = waitVisible(state, page, sel.String(), locator)
	if err != nil {
		return sel, nil, err
	}

	state.LastSelector = text

	return sel, locator, nil
}
