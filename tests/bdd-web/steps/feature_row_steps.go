package steps

import (
	"errors"
	"strings"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrRowPartMissing is returned when a row a clause names holds no visible part
// of the kind the clause is about.
var ErrRowPartMissing = errors.New("the row holds no such part")

// ErrNoRowKey is returned when a clause names a row without the attribute that
// says which record it stands for.
var ErrNoRowKey = errors.New("the row reference names no id")

const (
	// rowLinkCSS is a row's own link — its title, which is what a reader clicks.
	rowLinkCSS = "a[href]"
	// scenariosRoute is where a requirement's link lands, and descriptionField
	// the scalar a registry entry states its description in.
	scenariosRoute   = "/product/scenarios"
	descriptionField = "description"
)

// registerFeatureRowSteps binds a feature page's rows: what each row's link
// carries, that the link sits left of the row's own picker, and the click that
// follows it.
func registerFeatureRowSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^(`+selectorPattern+`) links to its story page carrying its id and title$`,
		assertRowLinksToStory)
	suite.Step(
		`^(`+selectorPattern+`) links to the scenarios page carrying its id and description$`,
		assertRowLinksToScenario)
	suite.Step(`^(`+selectorPattern+`) is a linked title left of its picker, sharing its row$`,
		assertLinkedTitleLeftOfPicker)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) clicks the link of (`+
			selectorPattern+`)$`,
		clickRowLink)
}

// assertRowLinksToStory holds a story row's link to reading the story's id and
// title and pointing at that story's own page — a row linking to a neighbour's
// page reads correctly and is still wrong.
func assertRowLinksToStory(state *State, args []string) error {
	sel, storyID, err := rowKey(state, args[0])
	if err != nil {
		return err
	}

	relPath, err := storyDocumentOf(state, storyID)
	if err != nil {
		return err
	}

	title, err := fixtureField(state, relPath, titleField)
	if err != nil {
		return err
	}

	return assertRowLink(state, sel, storyRoute+storyID, []string{storyID, title})
}

// assertRowLinksToScenario is the same clause for a requirement row, whose
// record is a registry entry rather than a document of its own.
func assertRowLinksToScenario(state *State, args []string) error {
	sel, scenarioID, err := rowKey(state, args[0])
	if err != nil {
		return err
	}

	description, err := entryField(state, scenarioID, descriptionField)
	if err != nil {
		return err
	}

	return assertRowLink(state, sel, scenariosRoute, []string{scenarioID, description})
}

// rowKey is the row a clause names and the id its reference keys it by: the row
// stands for one record, and the clause is about THAT record.
func rowKey(state *State, text string) (selector, string, error) {
	sel, err := parseSelector(text)
	if err != nil {
		return selector{}, "", state.fail("%w", err)
	}

	if sel.Value == "" {
		return selector{}, "", state.fail("%w: %s", ErrNoRowKey, sel)
	}

	return sel, sel.Value, nil
}

// entryField is one scalar of one registry entry of the connected project.
func entryField(state *State, scenarioID, field string) (string, error) {
	raw, err := fixtureFile(state, registryDocument)
	if err != nil {
		return "", err
	}

	block, err := entryBlock(raw, scenarioID)
	if err != nil {
		return "", state.fail("%s: %w", registryDocument, err)
	}

	value, err := scalarField(block, field)
	if err != nil {
		return "", state.fail("scenario %s of %s: %w", scenarioID, registryDocument, err)
	}

	return value, nil
}

// assertRowLink holds a row's link to pointing at a route and reading every
// fragment the record states, so a failure names what the row actually offered.
func assertRowLink(state *State, sel selector, route string, fragments []string) error {
	locator, err := sel.locate(state)
	if err != nil {
		return err
	}

	got, matched, err := await(readLinks(locator.Locator(rowLinkCSS)),
		linkHolding(route, fragments))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s holds the links %s, want one pointing at %s and reading %s",
			sel, summariseLines(got), route, strings.Join(fragments, " | "))
	}

	return nil
}

// linkHolding accepts a reading in which ONE link both points at the route and
// carries every fragment: two links each holding half is not a linked title.
func linkHolding(route string, fragments []string) func(string) bool {
	holds := containsAll(fragments)

	return func(value string) bool {
		for _, line := range strings.Split(value, "\n") {
			if strings.Contains(line, route) && holds(line) {
				return true
			}
		}

		return false
	}
}

// assertLinkedTitleLeftOfPicker holds a row to reading as one line: a linked
// title ending at or before the picker that tags it begins, and sharing its
// vertical span with it.
func assertLinkedTitleLeftOfPicker(state *State, args []string) error {
	sel, row, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	title, err := rowPartBox(state, sel, row.Locator(rowLinkCSS).First(), "link")
	if err != nil {
		return err
	}

	picker, err := rowPartBox(state, sel,
		row.Locator(elementCSS(pickerToggle, "", "")).First(), pickerToggle)
	if err != nil {
		return err
	}

	if title.X+title.Width > picker.X+edgeSlack {
		return state.fail("%s's link ends at %.1f and its %s begins at %.1f, want the link "+
			"to end at or before it", sel, title.X+title.Width, pickerToggle, picker.X)
	}

	if !sharesRow(title, picker) {
		return state.fail("%s's link spans %.1f-%.1f and its %s spans %.1f-%.1f, want them "+
			"on one row", sel, title.Y, title.Y+title.Height, pickerToggle,
			picker.Y, picker.Y+picker.Height)
	}

	return nil
}

// sharesRow answers whether two elements' vertical spans overlap, which is what
// a reader means by two things being on one row.
func sharesRow(first, second elementBox) bool {
	return first.Y < second.Y+second.Height-edgeSlack &&
		second.Y < first.Y+first.Height-edgeSlack
}

// rowPartBox measures one element inside a row, and names the row when the row
// does not hold it at all.
func rowPartBox(state *State, sel selector, locator playwright.Locator,
	part string,
) (elementBox, error) {
	err := locator.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(selectorTimeout),
	})
	if err != nil {
		return elementBox{}, state.fail("%w: %s holds no visible %s: %w",
			ErrRowPartMissing, sel, part, err)
	}

	box, err := locator.BoundingBox()
	if err != nil {
		return elementBox{}, state.fail("measuring %s's %s: %w", sel, part, err)
	}

	if box == nil {
		return elementBox{}, state.fail("%s's %s: %w", sel, part, ErrNoBoundingBox)
	}

	return boxOf(box), nil
}

// clickRowLink follows a row's own title link — the When behind a clause about
// where that title lands. args[0] is the role, discarded as clickElement's is.
func clickRowLink(state *State, args []string) error {
	rememberPageState(state)

	sel, row, err := locateStep(state, args[1])
	if err != nil {
		return err
	}

	err = row.Locator(rowLinkCSS).First().Click()
	if err != nil {
		return state.fail("clicking the link of %s: %w", sel, err)
	}

	return nil
}
