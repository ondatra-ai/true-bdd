package steps

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNothingHovered is returned when a clause is about what the pointer changed
// and no When hovered anything.
var ErrNothingHovered = errors.New("no step hovered an element")

// ErrNoFollowingElement is returned when a clause names the element following
// the hovered one and the page renders none after it.
var ErrNoFollowingElement = errors.New("nothing of that kind follows the hovered element")

const (
	// hoveredSubject and itSubject are how a clause names the element the pointer
	// is on: by what it is, or by "it".
	hoveredSubject = "the hovered link"
	itSubject      = "it"
	// followingPrefix opens the name of the element of one kind that comes after
	// it, and currentCrumbSuffix closes the name of a trail's own last crumb.
	followingPrefix    = "the following "
	currentCrumbSuffix = "'s current crumb"
	// colourProperty is the channel a hover answers on, and the one the clause
	// about a crumb that is not a link holds still.
	colourProperty = "color"
)

const (
	// pointerSubjectPattern is how a clause names the hovered element itself, and
	// hoverSubjectPattern adds the parts of the trail beside it.
	pointerSubjectPattern = hoveredSubject + `|` + itSubject
	hoverSubjectPattern   = pointerSubjectPattern + `|the following [a-z][a-z0-9-]*|` +
		selectorPattern + `'s current crumb`
)

// elementPathFunc answers with a CSS path naming one element and no other, the
// nth-child chain down from the body. A path IS a selector, so a clause can look
// the same element up again once the pointer has arrived.
const elementPathFunc = `((el) => {
	const parts = []
	for (let node = el; node && node.parentElement; node = node.parentElement) {
		const index = 1 + Array.prototype.indexOf.call(node.parentElement.children, node)
		parts.unshift(node.tagName.toLowerCase() + ":nth-child(" + index + ")")
	}
	return parts.join(" > ")
})`

// hoverSnapshotProbe records where every element sat BEFORE the pointer arrived,
// keyed by that path: a hover leaves nothing behind to measure afterwards.
const hoverSnapshotProbe = `() => JSON.stringify(Object.fromEntries(
	Array.from(document.querySelectorAll("body, body *")).map(el => {
		const box = el.getBoundingClientRect()
		return [` + elementPathFunc + `(el),
			{x: box.x, y: box.y, width: box.width, height: box.height}]
	})))`

// registerHoverSteps binds the pointer vocabulary: the two subjects a hover
// names in words, and the clauses about what that hover changed — its own box,
// the parts beside it, and the colour it answers on.
func registerHoverSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) hovers `+
		`the breadcrumb link "([^"]+)"$`, hoverBreadcrumbLink)
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) hovers `+
		`(`+selectorPattern+`)'s current crumb$`, hoverCurrentCrumb)
	suite.Step(`^(`+pointerSubjectPattern+`)'s width changed by no more than (\d+) pixels$`,
		assertHoveredWidthHeld)
	suite.Step(`^(`+hoverSubjectPattern+`) moved by no more than (\d+) pixels$`,
		assertSubjectHeldStill)
	suite.Step(`^(`+pointerSubjectPattern+`) resolves "([^"]*)" from the token "([^"]*)"$`,
		assertHoveredResolvesToken)
	suite.Step(`^hovering (`+pointerSubjectPattern+`) changes neither its colour `+
		`nor its geometry$`, assertHoverChangesNothing)
}

// hoverBreadcrumbLink puts the pointer on the trail's link reading the step's
// text, matched exactly: two crumbs may share a prefix.
func hoverBreadcrumbLink(state *State, args []string) error {
	_, trail, err := locateStep(state, breadcrumbTestID)
	if err != nil {
		return err
	}

	text := args[1]

	return hoverSubject(state, fmt.Sprintf("the breadcrumb link %q", text),
		trail.Locator(fmt.Sprintf(`%s:text-is(%q)`, trailLinkCSS, text)).First())
}

// hoverCurrentCrumb puts it on the crumb naming the page itself, which the
// clause about a hover answering with nothing is written about.
func hoverCurrentCrumb(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[1])
	if err != nil {
		return err
	}

	return hoverSubject(state, sel.String()+currentCrumbSuffix,
		locator.Locator(currentCrumbCSS).First())
}

// hoverSubject records what the page looked like before the pointer arrived and
// then hovers: the clauses after it are compared against that reading, and
// nothing else records it.
func hoverSubject(state *State, name string, locator playwright.Locator) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	err = locator.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(selectorTimeout),
	})
	if err != nil {
		return state.fail("the page never showed %s: %w\n%s", name, err, visibleText(page))
	}

	path, err := readElementPath(state, name, locator)
	if err != nil {
		return err
	}

	colour, err := readComputedStyle(locator, colourProperty)()
	if err != nil {
		return state.fail("%s: %w", name, err)
	}

	boxes, err := hoverSnapshot(state, page)
	if err != nil {
		return err
	}

	state.HoveredName, state.HoveredPath = name, path
	state.HoveredColour, state.HoverBoxes = colour, boxes

	err = locator.Hover()
	if err != nil {
		return state.fail("hovering %s: %w", name, err)
	}

	return nil
}

// hoverSnapshot is where every element sat at that one instant.
func hoverSnapshot(state *State, page playwright.Page) (map[string]elementBox, error) {
	raw, err := probeString(page, hoverSnapshotProbe)
	if err != nil {
		return nil, state.fail("measuring the page before the hover: %w", err)
	}

	boxes := map[string]elementBox{}

	err = json.Unmarshal([]byte(raw), &boxes)
	if err != nil {
		return nil, state.fail("decoding the page's boxes: %w\n%s", err, raw)
	}

	return boxes, nil
}

// readElementPath is the path naming one element, read through the browser so
// the snapshot's key and the lookup's key are computed by the same code.
func readElementPath(state *State, name string, locator playwright.Locator) (string, error) {
	path, err := readProbe(locator, `el => `+elementPathFunc+`(el)`)()
	if err != nil {
		return "", state.fail("locating %s on the page: %w", name, err)
	}

	return path, nil
}

// hoverSubjectLocator resolves what a clause about the hover names, and how a
// failure should call it.
func hoverSubjectLocator(state *State, text string) (string, playwright.Locator, error) {
	if text == hoveredSubject || text == itSubject {
		return hoveredElement(state)
	}

	kind, follows := strings.CutPrefix(text, followingPrefix)
	if follows {
		return followingElement(state, kind)
	}

	sel, locator, err := locateStep(state, strings.TrimSuffix(text, currentCrumbSuffix))
	if err != nil {
		return text, nil, err
	}

	return sel.String() + currentCrumbSuffix, locator.Locator(currentCrumbCSS).First(), nil
}

// hoveredElement is the element the pointer is on, resolved through the path the
// hover recorded: a locator kept across steps would name an element the page has
// since re-rendered.
func hoveredElement(state *State) (string, playwright.Locator, error) {
	page, err := state.page()
	if err != nil {
		return hoveredSubject, nil, err
	}

	if state.HoveredPath == "" {
		return hoveredSubject, nil, state.fail("%w", ErrNothingHovered)
	}

	return state.HoveredName, page.Locator(state.HoveredPath), nil
}

// followingElement is the first element of one kind AFTER the hovered one in
// document order: geometry alone could not tell two separators apart.
func followingElement(state *State, kind string) (string, playwright.Locator, error) {
	name := followingPrefix + kind

	page, err := state.page()
	if err != nil {
		return name, nil, err
	}

	if state.HoveredPath == "" {
		return name, nil, state.fail("%w", ErrNothingHovered)
	}

	path, err := probeString(page, followingPathProbe(state.HoveredPath, kind))
	if err != nil {
		return name, nil, state.fail("looking for %s: %w", name, err)
	}

	if path == "" {
		return name, nil, state.fail("%w: %s", ErrNoFollowingElement, name)
	}

	return name, page.Locator(path), nil
}

// followingPathProbe answers with that element's path, or an empty string when
// the page renders none after the hovered one.
func followingPathProbe(hoveredPath, kind string) string {
	return fmt.Sprintf(`() => {
		const anchor = document.querySelector(%q)
		if (!anchor) { return "" }
		const next = Array.from(document.querySelectorAll(%q)).find(el =>
			anchor.compareDocumentPosition(el) & Node.DOCUMENT_POSITION_FOLLOWING)
		return next ? %s(next) : ""
	}`, hoveredPath, elementCSS(kind, "", ""), elementPathFunc)
}

// hoverBoxBefore is where an element sat before the pointer arrived, read out of
// the snapshot the hover took.
func hoverBoxBefore(state *State, name string, locator playwright.Locator,
) (elementBox, error) {
	if state.HoverBoxes == nil {
		return elementBox{}, state.fail("%w, so there is nothing to compare %s to",
			ErrNothingHovered, name)
	}

	path, err := readElementPath(state, name, locator)
	if err != nil {
		return elementBox{}, err
	}

	box, measured := state.HoverBoxes[path]
	if !measured {
		return elementBox{}, state.fail("the page showed no %s before the hover, so "+
			"there is nothing to compare it to", name)
	}

	return box, nil
}

// assertHoveredWidthHeld holds the hovered element's width to the one it had
// before the pointer: hover feedback is colour, and a bolder face is a wider one.
func assertHoveredWidthHeld(state *State, args []string) error {
	name, locator, err := hoverSubjectLocator(state, args[0])
	if err != nil {
		return err
	}

	tolerance, err := pixels(state, args[1])
	if err != nil {
		return err
	}

	before, now, err := hoverBoxes(state, name, locator)
	if err != nil {
		return err
	}

	if math.Abs(now.Width-before.Width) <= tolerance {
		return nil
	}

	return state.fail("%s is %.1f pixels wide and was %.1f before the hover, want "+
		"them within %.0f pixels", name, now.Width, before.Width, tolerance)
}

// assertSubjectHeldStill holds one part of the trail to not moving, whatever the
// hover did to the element beside it.
func assertSubjectHeldStill(state *State, args []string) error {
	name, locator, err := hoverSubjectLocator(state, args[0])
	if err != nil {
		return err
	}

	tolerance, err := pixels(state, args[1])
	if err != nil {
		return err
	}

	before, now, err := hoverBoxes(state, name, locator)
	if err != nil {
		return err
	}

	if math.Abs(now.X-before.X) <= tolerance && math.Abs(now.Y-before.Y) <= tolerance {
		return nil
	}

	return state.fail("%s sits at (%.1f, %.1f) and sat at (%.1f, %.1f) before the "+
		"hover, want it within %.0f pixels",
		name, now.X, now.Y, before.X, before.Y, tolerance)
}

// hoverBoxes is that element's box before the pointer and under it — the two
// readings every clause above opens with.
func hoverBoxes(state *State, name string, locator playwright.Locator,
) (elementBox, elementBox, error) {
	before, err := hoverBoxBefore(state, name, locator)
	if err != nil {
		return elementBox{}, elementBox{}, err
	}

	now, err := locatorBox(state, name, locator)
	if err != nil {
		return before, elementBox{}, err
	}

	return before, now, nil
}

// assertHoveredResolvesToken holds what the hovered element paints to its token,
// through the comparison every other token clause shares.
func assertHoveredResolvesToken(state *State, args []string) error {
	name, locator, err := hoverSubjectLocator(state, args[0])
	if err != nil {
		return err
	}

	return assertTokenOn(state, name, locator, args[1], args[2])
}

// assertHoverChangesNothing holds the hovered element to answering with nothing
// at all: there is nothing there to press.
func assertHoverChangesNothing(state *State, args []string) error {
	name, locator, err := hoverSubjectLocator(state, args[0])
	if err != nil {
		return err
	}

	err = assertColourHeld(state, name, locator)
	if err != nil {
		return err
	}

	return assertBoxHeld(state, name, locator)
}

// assertColourHeld holds its text colour to the one it painted before the
// pointer arrived.
func assertColourHeld(state *State, name string, locator playwright.Locator) error {
	if state.HoveredColour == "" {
		return state.fail("%w", ErrNothingHovered)
	}

	got, err := readComputedStyle(locator, colourProperty)()
	if err != nil {
		return state.fail("%s: %w", name, err)
	}

	if got == state.HoveredColour {
		return nil
	}

	return state.fail("%s paints %s = %q under the pointer and painted %q before it, "+
		"want the hover to change neither its colour nor its geometry",
		name, colourProperty, got, state.HoveredColour)
}

// assertBoxHeld holds its box to the same, within the slack that keeps sub-pixel
// rounding from being a change.
func assertBoxHeld(state *State, name string, locator playwright.Locator) error {
	before, now, err := hoverBoxes(state, name, locator)
	if err != nil {
		return err
	}

	if boxHeld(before, now, edgeSlack) {
		return nil
	}

	return state.fail("%s covers %s under the pointer and covered %s before it, want "+
		"the hover to change neither its colour nor its geometry",
		name, renderBox(now), renderBox(before))
}

// boxHeld answers whether a box stayed put AND stayed the same size, within the
// slack the clause allows.
func boxHeld(before, now elementBox, tolerance float64) bool {
	return math.Abs(now.X-before.X) <= tolerance &&
		math.Abs(now.Y-before.Y) <= tolerance &&
		math.Abs(now.Width-before.Width) <= tolerance &&
		math.Abs(now.Height-before.Height) <= tolerance
}
