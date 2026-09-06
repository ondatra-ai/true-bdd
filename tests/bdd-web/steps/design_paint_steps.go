package steps

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// paintedWord is what the paint probe answers for a rule that actually
	// draws in its token's colour.
	paintedWord = "painted"
	// verticalAxis is the axis a padding clause names when it means the top
	// and bottom sides.
	verticalAxis = "vertical"
	// railItemTestID is the tile the rail's padding clause measures.
	railItemTestID = "rail-item"
	// paintFunc answers that word when the element renders a box AND paints a
	// channel — its own surface or a drawn border — in the colour the token
	// resolves to, and with what it read when it does not.
	paintFunc = `((el, token) => {
		const style = getComputedStyle(el)
		const raw = style.getPropertyValue(token).trim()
		if (raw === "") { return "no " + token + " that the page defines" }
		const probe = document.createElement("span")
		probe.style.backgroundColor = raw
		document.body.appendChild(probe)
		const want = getComputedStyle(probe).backgroundColor.trim()
		probe.remove()
		const box = el.getBoundingClientRect()
		if (box.width <= 0 || box.height <= 0) {
			return "a box of " + box.width + " by " + box.height
		}
		if (style.backgroundColor.trim() === want) { return %[1]q }
		const read = ["background-color: " + style.backgroundColor.trim()]
		for (const side of ["top", "right", "bottom", "left"]) {
			const width = parseFloat(style.getPropertyValue("border-" + side + "-width")) || 0
			const drawn = style.getPropertyValue("border-" + side + "-style") !== "none"
			const colour = style.getPropertyValue("border-" + side + "-color").trim()
			if (width > 0 && drawn && colour === want) { return %[1]q }
			read.push("border-" + side + ": " + width + "px " + colour)
		}
		return read.join(", ") + "; want " + want
	})`
)

// registerDesignPaintSteps binds the clauses about what the design actually
// paints and pads, and the three vocabularies landing beside it.
func registerDesignPaintSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^(`+selectorPattern+`) paints the token "([^"]*)"$`, assertPaintsToken)
	suite.Step(`^(`+selectorPattern+`) is padded "([^"]*)", "([^"]*)", `+
		`"([^"]*)", "([^"]*)"$`, assertPaddedTokens)
	suite.Step(`^a rail item's (vertical|horizontal) padding equals the token "([^"]*)"$`,
		assertRailItemPadding)
	registerSidebarEntrySteps(suite)
	registerElementBoundsSteps(suite)
	registerBreadcrumbShapeSteps(suite)
}

// assertPaintsToken holds a rule to actually drawing in its token's colour: a
// guide line with no box, or one painting nothing, is a line nobody sees.
func assertPaintsToken(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	token := args[1]

	got, matched, err := await(readProbe(locator, paintProbe(token)), equals(paintedWord))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s renders %s, want it %s in the colour of %s",
			sel, got, paintedWord, token)
	}

	return nil
}

// paintProbe binds that probe to one token, with the word it answers spliced
// in so the clause and the reading speak one vocabulary.
func paintProbe(token string) string {
	return fmt.Sprintf(`el => (%s)(el, %q)`, fmt.Sprintf(paintFunc, paintedWord), token)
}

// assertPaddedTokens holds all four paddings to the tokens the clause names in
// CSS order — top, right, bottom, left — so an indent taken from the wrong
// step of the spacing scale fails on the side it is wrong on.
func assertPaddedTokens(state *State, args []string) error {
	for index, property := range paddingSides() {
		err := assertLengthFromToken(state, args[0], property, args[index+1])
		if err != nil {
			return err
		}
	}

	return nil
}

// assertRailItemPadding holds a rail tile's padding on one axis to the token
// the design gives it, measured on the first tile: the clause is about the
// design every tile shares.
func assertRailItemPadding(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	axis, token := args[0], args[1]
	tile := page.Locator(elementCSS(railItemTestID, "", "")).First()

	for _, property := range axisPaddingSides(axis) {
		reading, matched, readErr := await(readLengthReading(tile, property, token),
			lengthsAgree)
		if readErr != nil {
			return state.fail("%s: %w", railItemTestID, readErr)
		}

		if !matched {
			got, want := renderReading(reading)

			return state.fail("%s has %s = %s, want the length of %s, which is %s",
				railItemTestID, property, got, token, want)
		}
	}

	return nil
}

// axisPaddingSides are the two properties one axis of a padding clause reads.
func axisPaddingSides(axis string) []string {
	if axis == verticalAxis {
		return []string{"padding-top", "padding-bottom"}
	}

	return []string{"padding-left", "padding-right"}
}
