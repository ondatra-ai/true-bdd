package steps

import (
	"errors"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoOpenStory is returned when a clause is about "the story" and the open
// page stands on no story route.
var ErrNoOpenStory = errors.New("the open page is not a story page")

// ErrMissingHeading is returned when the page holds none of the headings a
// clause lists.
var ErrMissingHeading = errors.New("the page holds no heading reading that")

const (
	// The file card's parts: a kicker over a title over a muted subtitle, and
	// under them the header bar naming the document and counting its lines.
	fileViewKicker    = "file-view-kicker"
	fileViewTitle     = "file-view-title"
	fileViewMeta      = "file-view-meta"
	fileViewHeader    = "file-view-header"
	fileViewPath      = "file-view-path"
	fileViewLineCount = "file-view-line-count"
	// storyRoute is what a story page's URL carries before the story's own id.
	storyRoute = "/product/stories/"
	// titleField is the scalar a story document states its title in.
	titleField = "title"
)

// registerFileCardSteps binds the header block every section renders its
// document under: the card itself, what a story page's title names, and the
// page-wide heading clauses beside it.
func registerFileCardSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the page renders the file card of "([^"]+)" titled "([^"]+)"$`, assertFileCard)
	suite.Step(`^(`+selectorPattern+`) names the story's id and title$`, assertNamesOpenStory)
	suite.Step(`^(`+selectorPattern+`) holds the story's feature$`, assertHoldsOpenStoryFeature)
	suite.Step(`^the page's heading has text "([^"]*)"$`, assertPageHeadingText)
	suite.Step(`^the page holds the headings ("[^"]*"(?:, "[^"]*")*)$`, assertPageHoldsHeadings)
	registerFileCardShapeSteps(suite)
	registerHeaderBlockSteps(suite)
}

// assertFileCard holds the page to rendering one document as a card: kicker,
// title and subtitle stacked in that order, then the header bar naming the
// document and counting its lines — the count read from the tree.
func assertFileCard(state *State, args []string) error {
	relPath, title := args[0], args[1]

	err := assertElementText(state, []string{fileViewTitle, title})
	if err != nil {
		return err
	}

	err = assertNonEmpty(state, []string{fileViewMeta})
	if err != nil {
		return err
	}

	err = assertStackedInOrder(state,
		[]string{fileViewKicker, fileViewTitle, fileViewMeta, fileViewHeader})
	if err != nil {
		return err
	}

	err = assertElementText(state, []string{fileViewPath, relPath})
	if err != nil {
		return err
	}

	return assertLineCountOf(state, relPath)
}

// assertStackedInOrder holds a run of elements to sitting one under the next, so
// the card's reading order is measured rather than read off the markup.
func assertStackedInOrder(state *State, testIDs []string) error {
	for index := 1; index < len(testIDs); index++ {
		err := assertSitsBelow(state, []string{testIDs[index], testIDs[index-1]})
		if err != nil {
			return err
		}
	}

	return nil
}

// assertLineCountOf holds the header bar's count to how many lines the document
// itself holds.
func assertLineCountOf(state *State, relPath string) error {
	raw, err := fixtureFile(state, relPath)
	if err != nil {
		return err
	}

	want := strconv.Itoa(lineCount(raw))

	sel, locator, err := locateStep(state, fileViewLineCount)
	if err != nil {
		return err
	}

	got, matched, err := await(readInnerText(locator), holdsNumber(want))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s reads %q, want the line count of %s, which is %s",
			sel, got, relPath, want)
	}

	return nil
}

// holdsNumber accepts a reading carrying the wanted number as a number of its
// own, so a count of 12 is not read out of "120".
func holdsNumber(want string) func(string) bool {
	// Compiled per call: a package-level regexp is a global.
	digits := regexp.MustCompile(`\d+`)

	return func(value string) bool {
		return slices.Contains(digits.FindAllString(value, -1), want)
	}
}

// assertNamesOpenStory holds an element to naming BOTH the id the route carries
// and the title the story document states.
func assertNamesOpenStory(state *State, args []string) error {
	storyID, relPath, err := openStoryDocument(state)
	if err != nil {
		return err
	}

	title, err := fixtureField(state, relPath, titleField)
	if err != nil {
		return err
	}

	return assertHolds(state, args[0], "the id and title of story "+storyID,
		[]string{storyID, title})
}

// assertHoldsOpenStoryFeature holds an element to carrying the feature the open
// story's own document declares, read from the tree rather than named by the
// scenario.
func assertHoldsOpenStoryFeature(state *State, args []string) error {
	storyID, relPath, err := openStoryDocument(state)
	if err != nil {
		return err
	}

	feature, err := fixtureField(state, relPath, featureField)
	if err != nil {
		return err
	}

	return assertHolds(state, args[0], "the feature of story "+storyID, []string{feature})
}

// openStoryDocument is the story the page stands on: its id, read off the route,
// and the document declaring it, resolved through the host's own stories
// directory so a renamed story file still resolves.
func openStoryDocument(state *State) (string, string, error) {
	page, err := state.page()
	if err != nil {
		return "", "", err
	}

	url := page.URL()

	_, tail, found := strings.Cut(url, storyRoute)
	if !found {
		return "", "", state.fail("%w: %s", ErrNoOpenStory, url)
	}

	storyID := routeSegment(tail)

	relPath, err := storyDocumentOf(state, storyID)
	if err != nil {
		return "", "", err
	}

	return storyID, relPath, nil
}

// routeSegment is the first path segment of a URL tail, without the query or
// fragment a link may carry after it.
func routeSegment(tail string) string {
	segment, _, _ := strings.Cut(tail, "/")
	segment, _, _ = strings.Cut(segment, "?")
	segment, _, _ = strings.Cut(segment, "#")

	return strings.TrimSpace(segment)
}

// storyDocumentOf is the document declaring one story id, which must resolve to
// exactly one file — as the engine's own resolver requires.
func storyDocumentOf(state *State, storyID string) (string, error) {
	storiesRel, err := storiesDir(state)
	if err != nil {
		return "", err
	}

	pattern := filepath.Join(state.Tree.Dir, storiesRel, storyID+"-*.yaml")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", state.fail("globbing %s: %w", pattern, err)
	}

	if len(matches) != 1 {
		return "", state.fail("%w: %s matched %d files, want exactly 1",
			ErrNoStoryFile, pattern, len(matches))
	}

	return filepath.ToSlash(filepath.Join(storiesRel, filepath.Base(matches[0]))), nil
}

// assertPageHeadingText holds ONE of the page's headings to reading exactly the
// step's text: which heading level carries it is the page's business.
func assertPageHeadingText(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	want := args[0]

	got, matched, err := await(readTexts(page.Locator(headingCSS)), holdsLine(want))
	if err != nil {
		return state.fail("reading the page's headings: %w", err)
	}

	if !matched {
		return state.fail("the page's headings read %s, want one reading %q",
			summariseLines(got), want)
	}

	return nil
}

// assertPageHoldsHeadings holds the page to carrying every heading the step
// lists — the clause a page gathering its rows under named sections is written
// with. Presence is the clause; order is not.
func assertPageHoldsHeadings(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	wanted := quotedList(args[0])

	got, matched, err := await(readTexts(page.Locator(headingCSS)), holdsEveryLine(wanted))
	if err != nil {
		return state.fail("reading the page's headings: %w", err)
	}

	if !matched {
		return state.fail("%w: the page's headings read %s, want %s",
			ErrMissingHeading, summariseLines(got), args[0])
	}

	return nil
}

// holdsEveryLine accepts a per-line reading holding each wanted line.
func holdsEveryLine(wanted []string) func(string) bool {
	return func(value string) bool {
		lines := strings.Split(value, "\n")

		for _, want := range wanted {
			if !slices.Contains(lines, want) {
				return false
			}
		}

		return true
	}
}

// quotedList is the entries of a step's `"a", "b"` list, unquoted.
func quotedList(text string) []string {
	// Compiled per call: a package-level regexp is a global.
	entry := regexp.MustCompile(`"([^"]*)"`)
	matches := entry.FindAllStringSubmatch(text, -1)
	values := make([]string, 0, len(matches))

	for _, match := range matches {
		values = append(values, match[1])
	}

	return values
}
