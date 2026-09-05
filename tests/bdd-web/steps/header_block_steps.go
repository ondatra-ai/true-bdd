package steps

import (
	"fmt"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// primaryTitleCSS is where a section page renders its own title: the testid
	// the file card gives it, and the content's first heading only when the page
	// renders neither — scoped to main, so the brand is not read as a title.
	primaryTitleCSS = `[data-testid="file-view-title"], [data-testid="page-title"], main h1`
	// kickerCSS is the eyebrow above that title under whichever name a page gives
	// it, and descriptionCSS the paragraph a landing describes itself in.
	kickerCSS      = `[data-testid$="kicker"]`
	descriptionCSS = `[data-testid$="description"], [data-testid="file-view-meta"]`
	// leadingPosition is the pseudo-element a clause names when it means the dot
	// a badge is led by.
	leadingPosition = "leading"
)

// pseudoContentFunc answers ok when the element generates a pseudo-element with
// content of its own — a dot drawn as an empty string is generated content — and
// with what it read when it does not.
const pseudoContentFunc = `((el, pseudo, ok) => {
	const content = getComputedStyle(el, pseudo).content
	if (content && content !== "none" && content !== "normal") { return ok }
	return "generates " + pseudo + " with content: " + content
})`

// registerHeaderBlockSteps binds the header block every section page carries:
// where its parts sit against each other, the badge one of them generates, and
// the titles and kickers a clause about EVERY section page is measured on.
func registerHeaderBlockSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^(`+selectorPattern+`) sits above (`+anchorPattern+`)$`, assertSitsAbove)
	suite.Step(`^(`+selectorPattern+`)'s centre lies within (`+selectorPattern+`)$`,
		assertCentreLiesWithin)
	suite.Step(`^(`+selectorPattern+`) generates a (leading|trailing) pseudo-element$`,
		assertGeneratesPseudoElement)
	suite.Step(`^every section page's primary title has computed "([^"]*)" = "([^"]*)"$`,
		assertEverySectionTitleComputed)
	suite.Step(`^the workspace "([^"]+)" shows a kicker above its heading$`,
		assertWorkspaceKickerAboveHeading)
	suite.Step(`^the workspace "([^"]+)" holds a non-empty description paragraph$`,
		assertWorkspaceHoldsDescription)
	registerControlAffordanceSteps(suite)
}

// sectionWorkspaces are the pages "every section page" names, in the vocabulary
// workspaceRoute already maps to routes.
func sectionWorkspaces() []string {
	return []string{"home", architectureNode, productNode, scenariosNode, "builds"}
}

// assertSitsAbove holds one element to ending where another begins — the mirror
// of the "sits below" clause, so a stacked block is measured rather than read
// off the markup's order.
func assertSitsAbove(state *State, args []string) error {
	sel, box, err := currentBox(state, args[0])
	if err != nil {
		return err
	}

	anchor, below, err := anchorBox(state, args[1])
	if err != nil {
		return err
	}

	if box.Y+box.Height <= below.Y+edgeSlack {
		return nil
	}

	return state.fail("%s ends at %.1f and %s begins at %.1f, want it to end at "+
		"or before that", sel, box.Y+box.Height, anchor, below.Y)
}

// assertCentreLiesWithin holds one element's own centre inside another's box, so
// "a badge in that bar" is a measurement rather than a reading of the nesting.
func assertCentreLiesWithin(state *State, args []string) error {
	sel, box, err := currentBox(state, args[0])
	if err != nil {
		return err
	}

	container, around, err := currentBox(state, args[1])
	if err != nil {
		return err
	}

	centre := elementBox{
		X: box.X + box.Width*centreFraction,
		Y: box.Y + box.Height*centreFraction,
	}

	if encloses(around, centre) {
		return nil
	}

	return state.fail("%s's centre is at (%.1f, %.1f) and %s covers %s, want the "+
		"centre inside it", sel, centre.X, centre.Y, container, renderBox(around))
}

// assertGeneratesPseudoElement holds an element to drawing the mark the design
// gives it in generated content: a dot typed into the text is not that mark.
func assertGeneratesPseudoElement(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	position := args[1]
	read := readProbe(locator, fmt.Sprintf(`el => %s(el, %q, %q)`,
		pseudoContentFunc, pseudoElementOf(position), verdictOK))

	got, matched, err := await(read, equals(verdictOK))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s %s, want it to generate a %s pseudo-element",
			sel, got, position)
	}

	return nil
}

// pseudoElementOf is the pseudo-element a clause's position names.
func pseudoElementOf(position string) string {
	if position == leadingPosition {
		return "::before"
	}

	return "::after"
}

// assertEverySectionTitleComputed holds every section page's own title to one
// computed property, walking the sections itself: the clause has no When, and a
// title read on one page says nothing about the next.
func assertEverySectionTitleComputed(state *State, args []string) error {
	property, want := args[0], args[1]

	for _, section := range sectionWorkspaces() {
		// The captured role openWorkspace discards; the name is what routes.
		err := openWorkspace(state, []string{"", section})
		if err != nil {
			return err
		}

		locator, err := firstVisible(state, primaryTitleCSS)
		if err != nil {
			return err
		}

		got, matched, err := await(readComputedStyle(locator, property), equals(want))
		if err != nil {
			return state.fail("the %q page's primary title: %w", section, err)
		}

		if !matched {
			return state.fail("the %q page's primary title has computed %s = %q, want %q",
				section, property, got, want)
		}
	}

	return nil
}

// assertWorkspaceKickerAboveHeading holds a landing to carrying the same eyebrow
// a file page does, above the title it names.
func assertWorkspaceKickerAboveHeading(state *State, args []string) error {
	section := args[0]

	err := openWorkspace(state, []string{"", section})
	if err != nil {
		return err
	}

	kicker, err := cssBox(state, kickerCSS)
	if err != nil {
		return err
	}

	heading, err := cssBox(state, primaryTitleCSS)
	if err != nil {
		return err
	}

	if kicker.Y+kicker.Height <= heading.Y+edgeSlack {
		return nil
	}

	return state.fail("the %q page's kicker ends at %.1f and its heading begins at "+
		"%.1f, want the kicker above it", section, kicker.Y+kicker.Height, heading.Y)
}

// assertWorkspaceHoldsDescription holds a landing to saying what it is for: a
// page that renders the paragraph empty says nothing.
func assertWorkspaceHoldsDescription(state *State, args []string) error {
	section := args[0]

	err := openWorkspace(state, []string{"", section})
	if err != nil {
		return err
	}

	locator, err := firstVisible(state, descriptionCSS)
	if err != nil {
		return err
	}

	got, matched, err := await(readInnerText(locator),
		func(value string) bool { return value != "" })
	if err != nil {
		return state.fail("the %q page's description: %w", section, err)
	}

	if !matched {
		return state.fail("the %q page's description paragraph reads %q, want some "+
			"text of its own", section, got)
	}

	return nil
}

// firstVisible is the first element a CSS this suite holds matches, waited for
// as every element clause waits.
func firstVisible(state *State, css string) (playwright.Locator, error) {
	page, err := state.page()
	if err != nil {
		return nil, err
	}

	locator := page.Locator(css).First()

	err = waitVisible(state, page, css, locator)
	if err != nil {
		return nil, err
	}

	return locator, nil
}

// waitVisible holds a resolved locator to being rendered, which is what every
// lookup in this suite does before reading anything off it.
func waitVisible(state *State, page playwright.Page, name string,
	locator playwright.Locator,
) error {
	err := locator.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(selectorTimeout),
	})
	if err != nil {
		return state.fail("the page never showed %s: %w\n%s", name, err, visibleText(page))
	}

	return nil
}
