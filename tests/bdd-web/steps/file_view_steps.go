package steps

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoSeededToken is returned when a clause is about the seeded token and no
// Given seeded one.
var ErrNoSeededToken = errors.New("no step seeded a token into a document")

// ErrNoMatchingLine is returned when the document a marking clause names holds
// no line matching the pattern it names.
var ErrNoMatchingLine = errors.New("the document has no line matching that pattern")

// ErrEntityFormOffered is returned when the page offers a per-entity form,
// which a workspace that edits documents as files must not.
var ErrEntityFormOffered = errors.New("the page offers a per-entity form page")

// ErrUnreadableProbe is returned when a browser probe answers something other
// than the string it is written to return.
var ErrUnreadableProbe = errors.New("the browser probe answered a non-string")

const (
	// multiAttributeChildPattern is the selector grammar with a child pinned by
	// TWO OR MORE attributes. Two is the floor, which is what keeps it disjoint
	// from keyedChildSelectorPattern.
	multiAttributeChildPattern = `[a-z][a-z0-9-]*(?:\[[a-z][a-z0-9-]*=[^\]]+\])?` +
		` > [a-z][a-z0-9-]*(?:\[[a-z][a-z0-9-]*=[^\]]+\]){2,}`
	// markedLineAttribute is where the flash renders the 1-based line it marks.
	markedLineAttribute = "data-line"
	// scrollEndSlack absorbs sub-pixel rounding when asking whether a scroller
	// stands at its end, and scrolledToEnd is what the reader answers when it does.
	scrollEndSlack = 2
	scrolledToEnd  = "at its end"
)

// entityFormProbe answers with the first per-entity form surface or link the
// page offers, and with an empty string when it offers none.
const entityFormProbe = `() => {
	const form = document.querySelector('[data-testid^="entity-form"],[data-testid$="-form"],[data-testid$="-form-page"]')
	if (form) { return form.getAttribute("data-testid") }
	const link = Array.from(document.querySelectorAll("a[href]"))
		.find(anchor => /\/(new|edit|form)(\/|$)/.test(anchor.getAttribute("href")))
	return link ? link.getAttribute("href") : ""
}`

// registerFileViewSteps binds the file page's vocabulary: the document a page
// renders, the gutter beside it, the line an outline entry marks, and the
// regexp form of a text clause.
func registerFileViewSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^a unique token is seeded into "([^"]+)"$`, seedUniqueToken)
	suite.Step(`^(`+selectorPattern+`) contains the seeded token$`, assertContainsSeededToken)
	suite.Step(`^the page shows one (`+selectorPattern+`) per line of "([^"]+)"$`,
		assertOneElementPerLine)
	suite.Step(`^the page offers no per-entity form page$`, assertNoEntityForm)
	suite.Step(`^(`+selectorPattern+`) matches (.+)$`, assertElementMatches)
	suite.Step(`^(`+selectorPattern+`) does not match (.+)$`, refuteElementMatches)
	suite.Step(`^(`+selectorPattern+`) marks the line matching (.+) in "([^"]+)"$`, assertMarksLine)
	suite.Step(`^(`+selectorPattern+`) is scrolled to its end$`, assertScrolledToEnd)
	suite.Step(`^the page shows (`+multiAttributeChildPattern+`)$`, assertMultiAttributeShown)
	// The keyed-child twins of clauses the one-attribute grammar already binds:
	// same assertion, disjoint pattern, because a bracketed child never matches
	// selectorPattern.
	suite.Step(`^the page shows exactly (\d+) (`+keyedChildSelectorPattern+`)$`, assertElementCount)
	suite.Step(`^(`+keyedChildSelectorPattern+`) contains text "([^"]*)"$`, assertElementContainsText)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) clicks (`+
			keyedChildSelectorPattern+`)$`,
		clickElement)
}

// seedUniqueToken writes a marker no other run can have produced into one
// document of the project tree, so a clause about the file's own content is
// about THIS file rather than about anything that looks like it.
func seedUniqueToken(state *State, args []string) error {
	relPath := args[0]
	token := fmt.Sprintf("tbdd-seed-%s-%d", state.Scenario.ID, time.Now().UnixNano())

	err := seedTokenInto(state, relPath, token)
	if err != nil {
		return err
	}

	state.SeededToken = token
	state.SeededPath = relPath

	return nil
}

// assertContainsSeededToken holds an element's text to carrying the marker the
// Given seeded, which is the containment clause under a different name.
func assertContainsSeededToken(state *State, args []string) error {
	if state.SeededToken == "" {
		return state.fail("%w", ErrNoSeededToken)
	}

	return assertElementContainsText(state, []string{args[0], state.SeededToken})
}

// assertOneElementPerLine holds how many elements the page renders to the
// document's own line count, read from the tree rather than from a number the
// scenario repeats.
func assertOneElementPerLine(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	sel, err := parseSelector(args[0])
	if err != nil {
		return state.fail("%w", err)
	}

	relPath := args[1]

	raw, err := fixtureFile(state, relPath)
	if err != nil {
		return err
	}

	want := strconv.Itoa(lineCount(raw))

	got, matched, err := await(readCount(page, sel), equals(want))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("the page shows %s %s, want one per line of %s, which has %s lines",
			got, sel, relPath, want)
	}

	return nil
}

// lineCount is how many lines a buffer holds: a trailing newline ends the last
// line rather than opening an empty one.
func lineCount(raw string) int {
	if raw == "" {
		return 0
	}

	return strings.Count(strings.TrimSuffix(raw, "\n"), "\n") + 1
}

// assertNoEntityForm holds the page to offering no per-entity form: this
// workspace edits documents as files, and a form page would be a second way to
// change the same document. Watched, so a late render cannot slip past.
func assertNoEntityForm(state *State, _ []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	deadline := time.Now().Add(attributeSettle)

	for {
		offender, probeErr := probeString(page, entityFormProbe)
		if probeErr != nil {
			return state.fail("looking for a per-entity form: %w", probeErr)
		}

		if offender != "" {
			return state.fail("%w: %s", ErrEntityFormOffered, offender)
		}

		if !time.Now().Before(deadline) {
			return nil
		}

		time.Sleep(valuePollInterval)
	}
}

// probeString runs a page probe written to answer a string, and says so when it
// answers anything else.
func probeString(page playwright.Page, script string) (string, error) {
	value, err := page.Evaluate(script)
	if err != nil {
		return "", fmt.Errorf("run a page probe: %w", err)
	}

	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%w: %v", ErrUnreadableProbe, value)
	}

	return text, nil
}

// assertElementMatches holds an element's rendered text to matching the step's
// regexp, which runs undelimited to the end of the line.
func assertElementMatches(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	pattern, err := regexp.Compile(args[1])
	if err != nil {
		return state.fail("the step's pattern %q does not compile: %w", args[1], err)
	}

	got, matched, err := await(readInnerText(locator), pattern.MatchString)
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s reads %q, which does not match %s", sel, got, pattern)
	}

	return nil
}

// refuteElementMatches is the negative twin, watched rather than read once: text
// that arrives a render late would pass a single read.
func refuteElementMatches(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	pattern, err := regexp.Compile(args[1])
	if err != nil {
		return state.fail("the step's pattern %q does not compile: %w", args[1], err)
	}

	read := readInnerText(locator)
	deadline := time.Now().Add(attributeSettle)

	for {
		got, readErr := read()
		if readErr != nil {
			return state.fail("%s: %w", sel, readErr)
		}

		if pattern.MatchString(got) {
			return state.fail("%s reads %q, which matches %s, want text that does not",
				sel, got, pattern)
		}

		if !time.Now().Before(deadline) {
			return nil
		}

		time.Sleep(valuePollInterval)
	}
}

// assertMarksLine holds the marker to the line the document itself puts the
// step's pattern on, so the expected number is read from the file rather than
// written into the scenario.
func assertMarksLine(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	expression, relPath := args[1], args[2]

	number, err := matchingLineNumber(state, relPath, expression)
	if err != nil {
		return err
	}

	want := strconv.Itoa(number)

	got, matched, err := await(readAttribute(locator, markedLineAttribute), equals(want))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s carries %s = %q, want %q: that is the first line of %s matching %s",
			sel, markedLineAttribute, got, want, relPath, expression)
	}

	return nil
}

// matchingLineNumber is the 1-based line of the first line of a document
// matching the step's pattern.
func matchingLineNumber(state *State, relPath, expression string) (int, error) {
	pattern, err := regexp.Compile(expression)
	if err != nil {
		return 0, state.fail("the step's pattern %q does not compile: %w", expression, err)
	}

	raw, err := fixtureFile(state, relPath)
	if err != nil {
		return 0, err
	}

	for index, line := range strings.Split(raw, "\n") {
		if pattern.MatchString(line) {
			return index + 1, nil
		}
	}

	return 0, state.fail("%w: %s in %s", ErrNoMatchingLine, expression, relPath)
}

// assertScrolledToEnd holds a scroller to standing at its own end. Polled: the
// page scrolls after the click returns, so the first read is mid-scroll.
func assertScrolledToEnd(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	got, matched, err := await(readScrollPosition(locator), equals(scrolledToEnd))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s is %s, want it %s", sel, got, scrolledToEnd)
	}

	return nil
}

// readScrollPosition answers with the words the clause uses when the scroller
// stands at its end, and with the numbers behind that judgement when it does not.
func readScrollPosition(locator playwright.Locator) func() (string, error) {
	script := fmt.Sprintf(
		`el => el.scrollTop + el.clientHeight >= el.scrollHeight - %d ? %q : `+
			`"scrolled to " + el.scrollTop + " of " + (el.scrollHeight - el.clientHeight)`,
		scrollEndSlack, scrolledToEnd)

	return func() (string, error) {
		value, err := locator.Evaluate(script, nil)
		if err != nil {
			return "", fmt.Errorf("read the element's scroll position: %w", err)
		}

		text, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("%w: %v", ErrUnreadableProbe, value)
		}

		return text, nil
	}
}

// assertMultiAttributeShown is the presence clause for a child pinned by more
// than one attribute: the parent parses as usual and the child's attributes are
// rendered as CSS here, so the shared grammar keeps its one-attribute shape.
func assertMultiAttributeShown(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	parentText, childText, _ := strings.Cut(args[0], " > ")

	parent, err := parseSelector(parentText)
	if err != nil {
		return state.fail("%w", err)
	}

	locator := parent.element(page).Locator(multiAttributeCSS(childText))

	err = locator.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(selectorTimeout),
	})
	if err != nil {
		return state.fail("the page never showed %s > %s: %w\n%s",
			parent, childText, err, visibleText(page))
	}

	return nil
}

// multiAttributeCSS renders `name[key=value][key=value]` as the CSS the UI's
// testid contract puts those attributes in.
func multiAttributeCSS(text string) string {
	name, rest, _ := strings.Cut(text, "[")
	css := fmt.Sprintf("[data-testid=%q]", name)
	// Compiled per call: a package-level regexp is a global.
	term := regexp.MustCompile(`\[([a-z][a-z0-9-]*)=([^\]]+)\]`)

	var cssSb394 strings.Builder
	for _, parts := range term.FindAllStringSubmatch("["+rest, -1) {
		fmt.Fprintf(&cssSb394, "[data-%s=%q]", parts[1], parts[2])
	}

	css += cssSb394.String()

	return css
}
