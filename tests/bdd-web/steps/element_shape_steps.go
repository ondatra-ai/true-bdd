package steps

import (
	"fmt"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// underlined is what the underline probe answers for an element carrying a
	// line under its text — drawn by text-decoration or by a bottom border,
	// which a reader cannot tell apart and this design system uses both of.
	underlined = "underlined"
	// brandNameAnchor and headingAnchor are the anchors a "sits below" clause
	// names in words, and brandNameCSS is where the brand's own name renders.
	brandNameAnchor = "the brand name"
	headingAnchor   = "the page's heading"
	brandNameCSS    = `[data-testid="sidebar-brand-name"]`
	// anchorPattern is what the second half of a "sits below" clause may be:
	// an element reference, or one of the two anchors named in words.
	anchorPattern = brandNameAnchor + `|` + headingAnchor + `|` + selectorPattern
)

// registerElementShapeSteps binds the clauses about one element's own shape:
// that it renders any text at all, that its text is underlined, and where it
// sits against another element.
func registerElementShapeSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^(`+selectorPattern+`) is non-empty$`, assertNonEmpty)
	suite.Step(`^(`+selectorPattern+`) is underlined$`, assertUnderlined)
	suite.Step(`^(`+selectorPattern+`) sits below (`+anchorPattern+`)$`, assertSitsBelow)
	suite.Step(`^(`+selectorPattern+`) begins at or after the right edge of `+
		`(`+selectorPattern+`)$`, assertBeginsAfterRightEdge)
	registerLastNamedSteps(suite)
}

// assertBeginsAfterRightEdge holds one element to starting where another ends
// across, so "floats clear of it" is a measurement rather than a reading of the
// markup's order.
func assertBeginsAfterRightEdge(state *State, args []string) error {
	sel, box, err := currentBox(state, args[0])
	if err != nil {
		return err
	}

	anchor, beside, err := currentBox(state, args[1])
	if err != nil {
		return err
	}

	if box.X >= beside.X+beside.Width-edgeSlack {
		return nil
	}

	return state.fail("%s begins at %.1f and %s's right edge is at %.1f, want it to "+
		"begin at or after that", sel, box.X, anchor, beside.X+beside.Width)
}

// assertNonEmpty holds an element to rendering text of its own: a clause about
// a line whose content the scenario cannot name still refuses an empty one.
func assertNonEmpty(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	got, matched, err := await(readInnerText(locator),
		func(value string) bool { return value != "" })
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s reads %q, want some text of its own", sel, got)
	}

	return nil
}

// assertUnderlined holds an element to carrying a line under its text.
func assertUnderlined(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	got, matched, err := await(readProbe(locator, underlineProbe()), equals(underlined))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s renders %s, want it %s", sel, got, underlined)
	}

	return nil
}

// underlineProbe answers with the word the clause uses when the element carries
// a line under its text, and with the declarations behind that judgement when
// it does not — so the failure names what the page actually painted.
func underlineProbe() string {
	return fmt.Sprintf(`el => {
		const style = getComputedStyle(el)
		const width = parseFloat(style.borderBottomWidth) || 0
		if (style.textDecorationLine.includes("underline")) { return %[1]q }
		if (style.borderBottomStyle !== "none" && width > 0) { return %[1]q }
		return "text-decoration-line: " + style.textDecorationLine +
			", border-bottom: " + style.borderBottomWidth + " " + style.borderBottomStyle
	}`, underlined)
}

// assertSitsBelow holds one element to beginning where another ends, so
// "beneath" is a measurement rather than a reading of the markup's order.
func assertSitsBelow(state *State, args []string) error {
	sel, box, err := currentBox(state, args[0])
	if err != nil {
		return err
	}

	anchor, above, err := anchorBox(state, args[1])
	if err != nil {
		return err
	}

	if box.Y >= above.Y+above.Height-edgeSlack {
		return nil
	}

	return state.fail("%s begins at %.1f and %s ends at %.1f, want it to begin at "+
		"or after that", sel, box.Y, anchor, above.Y+above.Height)
}

// anchorBox measures what a clause's second half names: an element reference
// resolves through the shared grammar, an anchor named in words through the
// CSS this suite holds for it.
func anchorBox(state *State, text string) (string, elementBox, error) {
	css, named := anchorCSS(text)
	if !named {
		sel, box, err := currentBox(state, text)

		return sel.String(), box, err
	}

	box, err := cssBox(state, css)

	return text, box, err
}

// anchorCSS is the CSS behind an anchor named in words, and whether the clause
// named one at all.
func anchorCSS(text string) (string, bool) {
	switch text {
	case brandNameAnchor:
		return brandNameCSS, true
	case headingAnchor:
		return headingCSS, true
	default:
		return "", false
	}
}

// cssBox measures the first element a CSS this suite holds matches, waiting for
// it as every element clause does.
func cssBox(state *State, css string) (elementBox, error) {
	locator, err := firstVisible(state, css)
	if err != nil {
		return elementBox{}, err
	}

	return locatorBox(state, css, locator)
}

// readProbe runs a probe written to answer a string about ONE element, so the
// shape and token clauses poll through the same await every value clause uses.
func readProbe(locator playwright.Locator, script string) func() (string, error) {
	return func() (string, error) {
		value, err := locator.Evaluate(script, nil)
		if err != nil {
			return "", fmt.Errorf("run an element probe: %w", err)
		}

		text, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("%w: %v", ErrUnreadableProbe, value)
		}

		return text, nil
	}
}
