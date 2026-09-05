package steps

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// topBarTestID, wordmarkTestID and taglineTestID are the frame contract for
	// the parts a clause names in words rather than through the selector grammar.
	topBarTestID   = "top-bar"
	wordmarkTestID = "wordmark"
	taglineTestID  = "tagline"
	// backgroundImageProperty is where a gradient token is painted.
	backgroundImageProperty = "background-image"
	// controlCSS is everything a reader can operate, which is what a clause about
	// a control the page must not offer is read over.
	controlCSS = "a, button, input, select, [role=button], [role=link]"
)

// registerSessionsFrameSteps binds the clauses about the frame the sessions home
// wears: its gradient bar, its tagline, how many top-level headings it sets, and
// the control it must not offer.
func registerSessionsFrameSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the page paints a top bar from the token "([^"]*)"$`, assertTopBarToken)
	suite.Step(`^the page holds a visible tagline matching (.+)$`, assertTaglineMatches)
	suite.Step(`^the page shows exactly (\d+) headings? of level ([1-6])$`,
		assertHeadingLevelCount)
	suite.Step(`^the page offers no ([a-z][a-z0-9-]*) control, by testid or by name$`,
		assertNoControlOffered)
}

// assertTopBarToken holds the bar's own paint to the value its token resolves to,
// through the probe every token clause reads a value with, so a bar hard-coding
// the same gradient by hand still fails.
func assertTopBarToken(state *State, args []string) error {
	token := args[0]

	locator, err := locateFrame(state, topBarTestID, "the top bar")
	if err != nil {
		return err
	}

	reading, matched, err := await(
		readTokenReading(locator, backgroundImageProperty, token), readingsAgree)
	if err != nil {
		return state.fail("the top bar: %w", err)
	}

	if !matched {
		got, want := renderReading(reading)

		return state.fail("the top bar has %s = %s, want the value of %s, which is %s",
			backgroundImageProperty, got, token, want)
	}

	return nil
}

// assertTaglineMatches holds the tagline to being rendered AND to reading as the
// clause's regexp, which runs undelimited to the end of the line.
func assertTaglineMatches(state *State, args []string) error {
	pattern, err := regexp.Compile(args[0])
	if err != nil {
		return state.fail("the step's pattern %q does not compile: %w", args[0], err)
	}

	locator, err := locateFrame(state, taglineTestID, "the tagline")
	if err != nil {
		return err
	}

	got, matched, err := await(readInnerText(locator), pattern.MatchString)
	if err != nil {
		return state.fail("reading the tagline: %w", err)
	}

	if !matched {
		return state.fail("the tagline reads %q, which does not match %s", got, pattern)
	}

	return nil
}

// assertHeadingLevelCount holds how many headings of one level the page sets: two
// first-level headings is two documents on one page, which is the clause.
func assertHeadingLevelCount(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	want, level := args[0], args[1]
	css := "h" + level

	got, matched, err := await(readCSSCount(page, css), equals(want))
	if err != nil {
		return state.fail("counting the page's level-%s headings: %w", level, err)
	}

	if !matched {
		reading, readErr := readTexts(page.Locator(css))()
		if readErr != nil {
			reading = ""
		}

		return state.fail("the page shows %s headings of level %s, want exactly %s; they read %s",
			got, level, want, summariseLines(reading))
	}

	return nil
}

// assertNoControlOffered holds the page to offering that control neither by
// testid nor under its own name, watched rather than read once: a control
// rendered a beat late would pass a single reading.
func assertNoControlOffered(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	name := args[0]
	deadline := time.Now().Add(attributeSettle)

	for {
		sighting, readErr := probeString(page, controlProbe(name))
		if readErr != nil {
			return state.fail("reading the page's controls: %w", readErr)
		}

		if sighting != "" {
			return state.fail("the page offers a %s control, want none: %s", name, sighting)
		}

		if !time.Now().Before(deadline) {
			return nil
		}

		time.Sleep(valuePollInterval)
	}
}

// controlProbe answers with the control the page offers under that name — by
// testid or by the words a reader sees — and with nothing when it offers none.
func controlProbe(name string) string {
	return fmt.Sprintf(`() => {
		if (document.querySelector(%[1]q)) { return "an element carries the testid" }
		const pattern = new RegExp(%[2]q, "i")
		for (const el of document.querySelectorAll(%[3]q)) {
			const label = (el.getAttribute("aria-label") || "") + " " +
				(el.getAttribute("value") || "") + " " + (el.textContent || "")
			if (pattern.test(label)) {
				return el.tagName.toLowerCase() + ": " + label.trim().replace(/\s+/g, " ")
			}
		}
		return ""
	}`, elementCSS(name, "", ""), namePattern(name), controlCSS)
}

// namePattern is the control's name as a reader may see it written: the hyphens a
// testid joins its words with stand for any separator at all.
func namePattern(name string) string {
	parts := strings.Split(name, "-")
	for index, part := range parts {
		parts[index] = regexp.QuoteMeta(part)
	}

	return strings.Join(parts, `[\s-]*`)
}

// locateFrame resolves a frame part a clause names in words, waiting for it as
// the selector grammar waits for every other element.
func locateFrame(state *State, testID, name string) (playwright.Locator, error) {
	page, err := state.page()
	if err != nil {
		return nil, err
	}

	locator := page.Locator(elementCSS(testID, "", "")).First()

	err = locator.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(selectorTimeout),
	})
	if err != nil {
		return nil, state.fail("the page never showed %s: %w\n%s", name, err, visibleText(page))
	}

	return locator, nil
}

// readCSSCount reads how many elements a CSS this suite holds matches, rendered
// as the digits a step writes.
func readCSSCount(page playwright.Page, css string) func() (string, error) {
	return func() (string, error) {
		count, err := page.Locator(css).Count()
		if err != nil {
			return "", fmt.Errorf("count the elements matching %s: %w", css, err)
		}

		return strconv.Itoa(count), nil
	}
}
