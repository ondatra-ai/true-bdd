package steps

import (
	"fmt"
	"math"
	"strconv"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// edgeSlack absorbs sub-pixel rounding: a tenth of a pixel is not a resize, and
// not an overlap either.
const edgeSlack = 1.0

// registerLayoutSteps binds the clauses about where the panes sit: what a When
// changed about a width, where one pane ends against another, and the computed
// property a docked panel may not carry.
func registerLayoutSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^(`+selectorPattern+`) is (narrower|wider) than it was$`,
		assertWidthChanged)
	suite.Step(`^(`+selectorPattern+`) is (\d+) pixels (wider|narrower) than it was, `+
		`within (\d+) pixels$`, assertWidthDelta)
	suite.Step(`^(`+selectorPattern+`) ends at or before (`+selectorPattern+`) begins$`,
		assertEndsBefore)
	suite.Step(`^(`+selectorPattern+`) has computed "([^"]*)" other than "([^"]*)"$`,
		refuteComputedStyle)
}

// assertWidthChanged holds one pane's width to having moved the way the clause
// names, against the snapshot the When took before it acted. Polled: the layout
// settles a frame after the click that changed it.
func assertWidthChanged(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	before, err := snapshotBox(state, sel)
	if err != nil {
		return err
	}

	direction := args[1]

	got, matched, err := await(readWidth(locator), func(value string) bool {
		width, parseErr := strconv.ParseFloat(value, 64)

		return parseErr == nil && movedAs(direction, before.Width, width)
	})
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s is %s pixels wide and was %.1f, want it %s",
			sel, got, before.Width, direction)
	}

	return nil
}

// assertWidthDelta holds that move to a distance, within the slack the clause
// allows — a drag lands where the pointer went, not to the pixel.
func assertWidthDelta(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	before, err := snapshotBox(state, sel)
	if err != nil {
		return err
	}

	want, tolerance, err := deltaWanted(state, before.Width, args[1], args[2], args[3])
	if err != nil {
		return err
	}

	got, matched, err := await(readWidth(locator), within(want, tolerance))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s is %s pixels wide and was %.1f, want %.1f ± %.1f "+
			"(%s pixels %s)", sel, got, before.Width, want, tolerance, args[1], args[2])
	}

	return nil
}

// deltaWanted is the width the clause asks for and the slack it allows.
func deltaWanted(state *State, before float64,
	distanceText, direction, toleranceText string,
) (float64, float64, error) {
	distance, err := pixels(state, distanceText)
	if err != nil {
		return 0, 0, err
	}

	tolerance, err := pixels(state, toleranceText)
	if err != nil {
		return 0, 0, err
	}

	if direction == "narrower" {
		return before - distance, tolerance, nil
	}

	return before + distance, tolerance, nil
}

// assertEndsBefore holds one pane to ending where the other starts: a DOCKED
// panel takes its width out of the layout rather than covering it.
func assertEndsBefore(state *State, args []string) error {
	first, left, err := currentBox(state, args[0])
	if err != nil {
		return err
	}

	second, right, err := currentBox(state, args[1])
	if err != nil {
		return err
	}

	if left.X+left.Width <= right.X+edgeSlack {
		return nil
	}

	return state.fail("%s ends at %.1f and %s begins at %.1f, want it to end at "+
		"or before that", first, left.X+left.Width, second, right.X)
}

// refuteComputedStyle holds one computed property to being anything BUT the
// value the clause forbids.
func refuteComputedStyle(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	property, unwanted := args[1], args[2]

	got, matched, err := await(readComputedStyle(locator, property),
		func(value string) bool { return value != unwanted })
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s has computed %s = %q, want anything other than %q",
			sel, property, got, unwanted)
	}

	return nil
}

// movedAs answers whether a width moved the way the clause names, past the
// slack that keeps sub-pixel rounding from being a change.
func movedAs(direction string, before, now float64) bool {
	if direction == "narrower" {
		return now < before-edgeSlack
	}

	return now > before+edgeSlack
}

// within accepts a reading that lands inside the tolerance the clause allows.
func within(want, tolerance float64) func(string) bool {
	return func(value string) bool {
		got, err := strconv.ParseFloat(value, 64)

		return err == nil && math.Abs(got-want) <= tolerance
	}
}

// pixels parses a step's pixel count, which its own \d+ capture guarantees.
func pixels(state *State, text string) (float64, error) {
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, state.fail("the step's pixel count %q does not parse: %w", text, err)
	}

	return value, nil
}

// readWidth reads one element's rendered width, so a geometry clause polls
// through the same await every value clause uses.
func readWidth(locator playwright.Locator) func() (string, error) {
	return func() (string, error) {
		box, err := locator.BoundingBox()
		if err != nil {
			return "", fmt.Errorf("measure the element: %w", err)
		}

		if box == nil {
			return "", ErrNoBoundingBox
		}

		return strconv.FormatFloat(box.Width, 'f', 1, 64), nil
	}
}
