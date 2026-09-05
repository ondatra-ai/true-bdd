package steps

import (
	"fmt"
	"strings"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// bandFinderFunc is the walk both band clauses share: the nearest surface at or
// above an element that actually paints, since a row's band may be drawn by the
// row itself or by a wrapper around it.
const bandFinderFunc = `((el) => {
	const paints = node => {
		const paint = getComputedStyle(node).backgroundColor.trim()
		return paint !== "" && paint !== "transparent" &&
			!paint.replace(/ /g, "").startsWith("rgba(0,0,0,0)")
	}
	let band = el
	while (band && !paints(band)) { band = band.parentElement }
	return band
})`

// noBandRead is what both band readings answer when nothing at or above the
// element paints; it parses as neither a colour nor a length.
const noBandRead = "no painted surface at or above it"

// registerSidebarBandSteps binds the clauses about the band a row sits on: the
// token painting it, and the width it must run.
func registerSidebarBandSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^(`+selectorPattern+`) sits on a band resolving from the token "([^"]*)"$`,
		assertSitsOnBand)
	suite.Step(`^(`+selectorPattern+`)'s band runs the full width of (`+selectorPattern+`)$`,
		assertBandRunsFullWidth)
	suite.Step(`^that band runs the full width of (`+selectorPattern+`)$`,
		assertThatBandRunsFullWidth)
}

// assertSitsOnBand holds the surface under an element to the value its token
// resolves to, through the comparison every other token clause shares.
func assertSitsOnBand(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	token := args[1]

	reading, matched, err := await(readBandPaint(locator, token), readingsAgree)
	if err != nil {
		return state.fail("%s's band: %w", sel, err)
	}

	if !matched {
		got, want := renderReading(reading)

		return state.fail("%s sits on a band painting %s, want the value of %s, "+
			"which is %s", sel, got, token, want)
	}

	return nil
}

// assertBandRunsFullWidth is the width clause on the band under the element the
// step names.
func assertBandRunsFullWidth(state *State, args []string) error {
	return holdBandToFullWidth(state, args[0], args[1])
}

// assertThatBandRunsFullWidth is the same clause about the band under the
// element the clause BEFORE it named, which is what "that band" means.
func assertThatBandRunsFullWidth(state *State, args []string) error {
	if state.LastSelector == "" {
		return state.fail("%w", ErrNoNamedElement)
	}

	return holdBandToFullWidth(state, state.LastSelector, args[0])
}

// holdBandToFullWidth holds the band at or above one element to reaching both
// edges of another: an inset band reads as a different component.
func holdBandToFullWidth(state *State, text, containerText string) error {
	sel, locator, err := locateStep(state, text)
	if err != nil {
		return err
	}

	container, outer, err := currentBox(state, containerText)
	if err != nil {
		return err
	}

	got, matched, err := await(readBandSpan(locator), spansAcross(outer))
	if err != nil {
		return state.fail("%s's band: %w", sel, err)
	}

	if !matched {
		return state.fail("%s's band runs %s, want the full width of %s, "+
			"which is %.1f to %.1f", sel, renderSpan(got), container,
			outer.X, outer.X+outer.Width)
	}

	return nil
}

// readBandPaint reads the band's own surface and the value the token resolves
// to, joined the way every token reading is, so one comparison serves both.
func readBandPaint(locator playwright.Locator, token string) func() (string, error) {
	return readProbe(locator, fmt.Sprintf(`el => {
		const band = %[1]s(el)
		const raw = getComputedStyle(el).getPropertyValue(%[2]q).trim()
		let want = ""
		if (raw !== "") {
			const probe = document.createElement("span")
			probe.style.backgroundColor = raw
			document.body.appendChild(probe)
			want = getComputedStyle(probe).backgroundColor.trim()
			probe.remove()
		}
		const got = band ? getComputedStyle(band).backgroundColor.trim() : %[3]q
		return got + " " + want
	}`, bandFinderFunc, token, noBandRead))
}

// readBandSpan reads where that band begins and ends across, joined the same
// way, so a width clause polls through the same await every value clause uses.
func readBandSpan(locator playwright.Locator) func() (string, error) {
	return readProbe(locator, fmt.Sprintf(`el => {
		const band = %[1]s(el)
		if (!band) { return %[2]q + " " }
		const box = band.getBoundingClientRect()
		return box.left + " " + box.right
	}`, bandFinderFunc, noBandRead))
}

// spansAcross accepts a band reading whose edges reach the container's own, past
// the slack that keeps sub-pixel rounding from being an inset.
func spansAcross(outer elementBox) func(string) bool {
	return func(reading string) bool {
		leftText, rightText, _ := strings.Cut(reading, linkFieldSeparator)

		left, leftRead := parsePixels(leftText)
		right, rightRead := parsePixels(rightText)

		return leftRead && rightRead &&
			left <= outer.X+edgeSlack && right >= outer.X+outer.Width-edgeSlack
	}
}

// renderSpan renders a band reading for a failure, naming the absence when
// nothing at or above the element paints.
func renderSpan(reading string) string {
	leftText, rightText, _ := strings.Cut(reading, linkFieldSeparator)

	_, read := parsePixels(rightText)
	if !read {
		return leftText
	}

	return leftText + " to " + rightText
}
