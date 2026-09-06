package steps

import (
	"fmt"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// fileCardAnchor is the box a clause names in words rather than by testid,
	// and fileCardCSS where it renders, under either name the card carries.
	fileCardAnchor = "the file card"
	fileCardCSS    = `[data-testid="file-card"], [data-testid="file-view"]`
	// cardAnchorPattern is what a card clause's subject may be: that box, or an
	// element reference in the grammar every other clause reads.
	cardAnchorPattern = fileCardAnchor + `|` + selectorPattern
	// cardLineCSS is where a card's lines render, and cardTailCSS the pane
	// holding them, which a card rendering no line row is measured against.
	cardLineCSS = `[data-testid="file-view-gutter-line"]`
	cardTailCSS = `[data-testid="file-view-editor"]`
	// noLineRead is what the trail probe answers for a card with nothing to
	// measure against; it parses as no length, and so satisfies nothing.
	noLineRead = "no line at all"
)

// edgeColourFunc answers ok when all four borders are drawn in the colour the
// token resolves to, and with the sides that are not when they are not. The
// token goes through a probe because "#111" is not what a border reads back.
const edgeColourFunc = `((el, token, ok) => {
	const style = getComputedStyle(el)
	const raw = style.getPropertyValue(token).trim()
	if (raw === "") { return "no " + token + " that the page defines" }
	const probe = document.createElement("span")
	probe.style.backgroundColor = raw
	document.body.appendChild(probe)
	const want = getComputedStyle(probe).backgroundColor.trim()
	probe.remove()
	const wrong = []
	for (const side of ["top", "right", "bottom", "left"]) {
		const width = parseFloat(style.getPropertyValue("border-" + side + "-width")) || 0
		const drawn = style.getPropertyValue("border-" + side + "-style").trim()
		const colour = style.getPropertyValue("border-" + side + "-color").trim()
		if (width > 0 && drawn !== "none" && colour === want) { continue }
		wrong.push(side + ": " + width + "px " + drawn + " " + colour)
	}
	return wrong.length === 0 ? ok : wrong.join(", ") + "; want " + want
})`

// edgeWidthFunc answers ok when every edge the element actually draws is the
// token's own length thick. A box drawing no edge at all reads as a complaint
// rather than passing vacuously.
const edgeWidthFunc = `((el, token, ok, slack) => {
	const style = getComputedStyle(el)
	const raw = style.getPropertyValue(token).trim()
	if (raw === "") { return "no " + token + " that the page defines" }
	const probe = document.createElement("span")
	probe.style.borderStyle = "solid"
	probe.style.borderTopWidth = raw
	document.body.appendChild(probe)
	const want = parseFloat(getComputedStyle(probe).borderTopWidth) || 0
	probe.remove()
	const wrong = []
	let edges = 0
	for (const side of ["top", "right", "bottom", "left"]) {
		const width = parseFloat(style.getPropertyValue("border-" + side + "-width")) || 0
		const drawn = style.getPropertyValue("border-" + side + "-style").trim()
		if (width <= 0 || drawn === "none") { continue }
		edges++
		if (Math.abs(width - want) > slack) { wrong.push(side + " is " + width + "px") }
	}
	if (edges === 0) { return "no edge at all" }
	return wrong.length === 0 ? ok : wrong.join(", ") + "; want " + want + "px"
})`

// cardTrailFunc answers how far the box's own bottom falls below its last line,
// or the word the clause reads when it renders no line to measure against.
const cardTrailFunc = `((el, lineCSS, tailCSS, noLine) => {
	let lines = Array.from(el.querySelectorAll(lineCSS))
	if (lines.length === 0) { lines = Array.from(el.querySelectorAll(tailCSS)) }
	if (lines.length === 0) { return noLine }
	const last = Math.max(...lines.map(line => line.getBoundingClientRect().bottom))
	return String(el.getBoundingClientRect().bottom - last)
})`

// registerFileCardShapeSteps binds the clauses about the card as a box: the
// outline it draws, how thick that outline is, and how far it trails below the
// last line it holds.
func registerFileCardShapeSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^every edge of (`+cardAnchorPattern+`) draws the token "([^"]*)"$`,
		assertEveryEdgeDrawsToken)
	suite.Step(`^every strong edge of (`+cardAnchorPattern+`) is exactly the token `+
		`"([^"]*)" thick$`, assertStrongEdgesAreTokenThick)
	suite.Step(`^(`+cardAnchorPattern+`) trails no more than (\d+) pixels `+
		`below its last line$`, assertTrailsBelowLastLine)
}

// assertEveryEdgeDrawsToken holds all four of a box's borders to the colour its
// token resolves to: an outline missing a side is not the box the design draws.
func assertEveryEdgeDrawsToken(state *State, args []string) error {
	subject, locator, err := locateCardSubject(state, args[0])
	if err != nil {
		return err
	}

	token := args[1]
	read := readProbe(locator, fmt.Sprintf(`el => %s(el, %q, %q)`,
		edgeColourFunc, token, verdictOK))

	got, matched, err := await(read, equals(verdictOK))
	if err != nil {
		return state.fail("%s: %w", subject, err)
	}

	if !matched {
		return state.fail("%s draws %s, want every edge in the colour of %s",
			subject, got, token)
	}

	return nil
}

// assertStrongEdgesAreTokenThick holds every edge the box actually draws to the
// token's own thickness, so an outline taken from a heavier variant fails on
// the side it is wrong on.
func assertStrongEdgesAreTokenThick(state *State, args []string) error {
	subject, locator, err := locateCardSubject(state, args[0])
	if err != nil {
		return err
	}

	token := args[1]
	read := readProbe(locator, fmt.Sprintf(`el => %s(el, %q, %q, %g)`,
		edgeWidthFunc, token, verdictOK, lengthSlack))

	got, matched, err := await(read, equals(verdictOK))
	if err != nil {
		return state.fail("%s: %w", subject, err)
	}

	if !matched {
		return state.fail("%s draws %s, want every edge it draws to be the length of %s",
			subject, got, token)
	}

	return nil
}

// assertTrailsBelowLastLine holds the box to ending where its content does: an
// empty run under the last line reads as a box with a hole in it.
func assertTrailsBelowLastLine(state *State, args []string) error {
	subject, locator, err := locateCardSubject(state, args[0])
	if err != nil {
		return err
	}

	want, err := pixels(state, args[1])
	if err != nil {
		return err
	}

	read := readProbe(locator, fmt.Sprintf(`el => %s(el, %q, %q, %q)`,
		cardTrailFunc, cardLineCSS, cardTailCSS, noLineRead))

	got, matched, err := await(read, trailsAtMost(want))
	if err != nil {
		return state.fail("%s: %w", subject, err)
	}

	if !matched {
		return state.fail("%s ends %s below its last line, want no more than %.0f pixels",
			subject, renderTrail(got), want)
	}

	return nil
}

// trailsAtMost accepts a trail no deeper than the clause's ceiling, past the
// slack that keeps sub-pixel rounding from being a gap.
func trailsAtMost(want float64) func(string) bool {
	return func(value string) bool {
		trail, read := parsePixels(value)

		return read && trail <= want+lengthSlack
	}
}

// renderTrail renders a trail reading for a failure: a length in pixels, or the
// complaint the probe answered in its place.
func renderTrail(reading string) string {
	_, read := parsePixels(reading)
	if !read {
		return reading
	}

	return reading + " pixels"
}

// locateCardSubject resolves what a card clause's subject names: the card named
// in words through the CSS this suite holds for it, or an element reference
// through the grammar every other clause reads.
func locateCardSubject(state *State, text string) (string, playwright.Locator, error) {
	if text != fileCardAnchor {
		sel, locator, err := locateStep(state, text)

		return sel.String(), locator, err
	}

	page, err := state.page()
	if err != nil {
		return text, nil, err
	}

	locator := page.Locator(fileCardCSS).First()

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
