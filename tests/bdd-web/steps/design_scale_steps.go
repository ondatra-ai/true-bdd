package steps

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// fontSizeProperty is what a "renders at" clause measures: the density
	// layer's scale reaches an element through its own font-size.
	fontSizeProperty = "font-size"
	// fontWeightProperty is what the weight clause reads.
	fontWeightProperty = "font-weight"
	// boldFloor is the computed weight at which a face reads bold: this design
	// system's semibold is 600, and a reader cannot tell 600 from 700.
	boldFloor = 600
	// boldWord is the polarity the weight clause captures when it asks for a
	// bold face; the other polarity the pattern allows is its negative.
	boldWord = "bold"
	// axisWidth and axisHeight name both the CSS property the token probe is
	// given and the rectangle field it is measured on — one word does each.
	axisWidth  = "width"
	axisHeight = "height"
)

// boxTokenReadingFunc answers with an element's rendered length along one axis
// and the length its token resolves to, joined by the NUL readLinks reserves.
// The probe is a block: a width set on an inline span computes to auto.
const boxTokenReadingFunc = `((el, axis, token) => {
	const style = getComputedStyle(el)
	const raw = style.getPropertyValue(token).trim()
	const got = String(el.getBoundingClientRect()[axis])
	if (raw === "") { return got + "\u0000" }
	const probe = document.createElement("div")
	probe.style.position = "absolute"
	probe.style.visibility = "hidden"
	probe.style.fontSize = style.fontSize
	probe.style.setProperty(axis, raw)
	document.body.appendChild(probe)
	const want = String(probe.getBoundingClientRect()[axis])
	probe.remove()
	return got + "\u0000" + want
})`

// registerDesignScaleSteps binds the clauses holding the chrome to the density
// layer's own scale: the size its text is set at, the frame's width and height,
// the weight of a face, and the rule a tree row must not carry.
func registerDesignScaleSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^(`+selectorPattern+`) renders at the token "([^"]*)"$`,
		assertRendersAtToken)
	suite.Step(`^(`+selectorPattern+`) is as wide as the token "([^"]*)"$`,
		assertWideAsToken)
	suite.Step(`^(`+selectorPattern+`) is as tall as the token "([^"]*)", `+
		`and no more than (\d+) pixels taller$`, assertTallAsToken)
	suite.Step(`^(`+selectorPattern+`) is (bold|not bold)$`, assertWeight)
	suite.Step(`^(`+selectorPattern+`) carries no bottom border rule$`,
		assertNoBottomRule)
	registerDesignPaintSteps(suite)
}

// assertRendersAtToken holds the size an element's text is set at to its
// token's length, through the same comparison every other length clause uses:
// a size that happens to match the token by hand still fails.
func assertRendersAtToken(state *State, args []string) error {
	return assertLengthFromToken(state, args[0], fontSizeProperty, args[1])
}

// assertWideAsToken holds one element's rendered width to the length its token
// resolves to, measured off the box rather than off the declaration: a frame
// the layout overrode is still the wrong width.
func assertWideAsToken(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	token := args[1]

	reading, matched, err := await(readBoxTokenReading(locator, axisWidth, token),
		lengthsAgree)
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		got, want := renderReading(reading)

		return state.fail("%s is %s pixels wide, want the length of %s, which is %s",
			sel, got, token, want)
	}

	return nil
}

// assertTallAsToken holds that measurement on the other axis, with a ceiling
// above it: a bar may grow with the padding around its text, but not into a
// different design.
func assertTallAsToken(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	token := args[1]

	slack, err := pixels(state, args[2])
	if err != nil {
		return err
	}

	reading, matched, err := await(readBoxTokenReading(locator, axisHeight, token),
		lengthsWithin(slack))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		got, want := renderReading(reading)

		return state.fail("%s is %s pixels tall, want the length of %s, which is %s, "+
			"and at most %.0f pixels more", sel, got, token, want, slack)
	}

	return nil
}

// assertWeight holds a face to the side of the bold floor the clause names, in
// either polarity: one definition serves "is bold" and "is not bold".
func assertWeight(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	polarity := args[1]

	got, matched, err := await(readComputedStyle(locator, fontWeightProperty),
		weighsAs(polarity))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s has %s = %q, want it %s (bold is %d or more)",
			sel, fontWeightProperty, got, polarity, boldFloor)
	}

	return nil
}

// assertNoBottomRule holds a row to carrying no rule under it: rows separated
// by a line are a different design from rows separated by space.
func assertNoBottomRule(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	got, matched, err := await(readComputedStyle(locator, bottomBorderWidthProperty),
		isZeroLength)
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s has %s = %q, want no rule under it at all",
			sel, bottomBorderWidthProperty, got)
	}

	return nil
}

// readBoxTokenReading reads that pair as a reader, so a box clause polls
// through the same await every value clause uses.
func readBoxTokenReading(locator playwright.Locator, axis, token string,
) func() (string, error) {
	return readProbe(locator, fmt.Sprintf(`el => %s(el, %q, %q)`,
		boxTokenReadingFunc, axis, token))
}

// lengthsWithin accepts a reading no shorter than its token's length and no
// more than the clause's own slack above it. A token the page does not define
// reads as no length at all, and satisfies nothing.
func lengthsWithin(slack float64) func(string) bool {
	return func(reading string) bool {
		got, want, _ := strings.Cut(reading, linkFieldSeparator)

		gotPixels, gotRead := parsePixels(got)
		wantPixels, wantRead := parsePixels(want)

		return gotRead && wantRead &&
			gotPixels >= wantPixels-lengthSlack && gotPixels <= wantPixels+slack
	}
}

// weighsAs accepts a computed weight on the side of the bold floor the clause
// names. A weight that does not read as a number matches neither polarity.
func weighsAs(polarity string) func(string) bool {
	return func(value string) bool {
		weight, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return false
		}

		if polarity == boldWord {
			return weight >= boldFloor
		}

		return weight < boldFloor
	}
}

// isZeroLength accepts a resolved length of zero, which is what a border
// declared 0 computes to and what one with no style computes to as well.
func isZeroLength(value string) bool {
	length, read := parsePixels(value)

	return read && length == 0
}
