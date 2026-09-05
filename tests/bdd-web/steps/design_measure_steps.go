package steps

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// bodyAnchor is what a clause names when its subject is the document's own
	// body rather than a testid.
	bodyAnchor = "the body"
	// fontFamilyProperty and bottomBorderWidthProperty are the computed
	// properties the face clause and the hairline clause read.
	fontFamilyProperty        = "font-family"
	bottomBorderWidthProperty = "border-bottom-width"
	// lengthSlack absorbs the rounding a resolved length lands on: half a pixel
	// is not a different token.
	lengthSlack = 0.5
)

// lengthReadingFunc answers with an element's own resolved length for a property
// and the length its token resolves to, joined by the NUL readLinks reserves.
// The probe carries a border style because a width with none computes to zero.
const lengthReadingFunc = `((el, property, token) => {
	const style = getComputedStyle(el)
	const got = style.getPropertyValue(property).trim()
	const raw = style.getPropertyValue(token).trim()
	if (raw === "") { return got + "\u0000" }
	const probe = document.createElement("span")
	probe.style.fontSize = style.fontSize
	probe.style.borderStyle = "solid"
	probe.style.setProperty(property, raw)
	document.body.appendChild(probe)
	const want = getComputedStyle(probe).getPropertyValue(property).trim()
	probe.remove()
	return got + "\u0000" + want
})`

// registerDesignMeasureSteps binds the clauses holding one element to a number
// the design gives it: its width, where it sits in its panel, the face it is set
// in, and the tokens its spacing and its hairline are drawn at.
func registerDesignMeasureSteps(suite *bddgo.Suite[State]) {
	// The rail's "lower part" under the chat's wording: one measurement, two
	// phrasings, so the definition is reused rather than copied.
	suite.Step(`^(`+selectorPattern+`) sits in the lower half of (`+selectorPattern+`)$`,
		assertSitsInLowerPart)
	suite.Step(`^(`+selectorPattern+`) is (\d+) pixels wide, within (\d+) pixels$`,
		assertWidthIs)
	suite.Step(`^(`+bodyAnchor+`|`+selectorPattern+`)'s font family matches (.+)$`,
		assertFontFamilyMatches)
	suite.Step(`^(`+selectorPattern+`)'s padding equals the token "([^"]*)" `+
		`on all four sides$`, assertPaddingEqualsToken)
	suite.Step(`^(`+selectorPattern+`)'s padding equals the token "([^"]*)" `+
		`at the (top|right|bottom|left)$`, assertPaddingSideEqualsToken)
	suite.Step(`^(`+selectorPattern+`)'s bottom border width equals the token "([^"]*)"$`,
		assertBottomBorderWidth)
}

// paddingSides are the four properties an "on all four sides" clause is about.
func paddingSides() []string {
	return []string{"padding-top", "padding-right", "padding-bottom", "padding-left"}
}

// assertWidthIs holds one element to the width the design gives it, within the
// slack the clause allows: a laid-out panel lands on the design's number, not on
// the pixel.
func assertWidthIs(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	want, err := pixels(state, args[1])
	if err != nil {
		return err
	}

	tolerance, err := pixels(state, args[2])
	if err != nil {
		return err
	}

	got, matched, err := await(readWidth(locator), within(want, tolerance))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s is %s pixels wide, want %.1f ± %.1f",
			sel, got, want, tolerance)
	}

	return nil
}

// assertFontFamilyMatches holds the face an element is set in to the step's
// regexp, which runs undelimited to the end of the line. Matched without regard
// to case: how a face's name is cased is not the reader's business.
func assertFontFamilyMatches(state *State, args []string) error {
	subject, locator, err := locateSubject(state, args[0])
	if err != nil {
		return err
	}

	pattern, err := regexp.Compile(`(?i)` + args[1])
	if err != nil {
		return state.fail("the step's pattern %q does not compile: %w", args[1], err)
	}

	read := readComputedStyle(locator, fontFamilyProperty)

	got, matched, err := await(read, pattern.MatchString)
	if err != nil {
		return state.fail("%s: %w", subject, err)
	}

	if !matched {
		return state.fail("%s is set in %q, which does not match %s", subject, got, pattern)
	}

	return nil
}

// assertPaddingEqualsToken holds all four paddings to the length the token
// resolves to, so a canvas padded by a number that happens to match still fails.
func assertPaddingEqualsToken(state *State, args []string) error {
	for _, property := range paddingSides() {
		err := assertLengthFromToken(state, args[0], property, args[1])
		if err != nil {
			return err
		}
	}

	return nil
}

// assertPaddingSideEqualsToken is that measurement on ONE side, which a design
// padding a pane differently across and down is stated in.
func assertPaddingSideEqualsToken(state *State, args []string) error {
	return assertLengthFromToken(state, args[0], "padding-"+args[2], args[1])
}

// assertBottomBorderWidth is that measurement on the hairline an element is
// ruled with.
func assertBottomBorderWidth(state *State, args []string) error {
	return assertLengthFromToken(state, args[0], bottomBorderWidthProperty, args[1])
}

// assertLengthFromToken holds one resolved length to its token's own, compared
// as numbers: "1px" and "1.0000px" are one length and two strings.
func assertLengthFromToken(state *State, text, property, token string) error {
	sel, locator, err := locateStep(state, text)
	if err != nil {
		return err
	}

	read := readLengthReading(locator, property, token)

	reading, matched, err := await(read, lengthsAgree)
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		got, want := renderReading(reading)

		return state.fail("%s has %s = %s, want the length of %s, which is %s",
			sel, property, got, token, want)
	}

	return nil
}

// locateSubject resolves what a clause's subject names: the document's own body,
// or an element reference through the grammar every other clause reads.
func locateSubject(state *State, text string) (string, playwright.Locator, error) {
	if text != bodyAnchor {
		sel, locator, err := locateStep(state, text)

		return sel.String(), locator, err
	}

	page, err := state.page()
	if err != nil {
		return bodyAnchor, nil, err
	}

	return bodyAnchor, page.Locator(bodyKey).First(), nil
}

// readLengthReading reads that pair as a reader, so a length clause polls
// through the same await every value clause uses.
func readLengthReading(locator playwright.Locator, property, token string,
) func() (string, error) {
	return readProbe(locator, fmt.Sprintf(`el => %s(el, %q, %q)`,
		lengthReadingFunc, property, token))
}

// lengthsAgree accepts a paired reading whose halves are the same length. A
// token the page does not define reads as no length at all, and agrees with
// nothing.
func lengthsAgree(reading string) bool {
	got, want, _ := strings.Cut(reading, linkFieldSeparator)

	gotPixels, gotRead := parsePixels(got)
	wantPixels, wantRead := parsePixels(want)

	return gotRead && wantRead && math.Abs(gotPixels-wantPixels) <= lengthSlack
}

// parsePixels reads a resolved length's number, and whether it read as one.
func parsePixels(text string) (float64, bool) {
	value, err := strconv.ParseFloat(strings.TrimSuffix(text, "px"), 64)

	return value, err == nil
}
