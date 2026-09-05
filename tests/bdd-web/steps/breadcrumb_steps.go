package steps

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// currentCrumbCSS is the crumb naming the page itself. The trail stamps it
	// aria-current rather than linking it (services/bdd-web/design/SPEC.md §1);
	// the testid form is accepted beside that stamp.
	currentCrumbCSS = `[aria-current="page"], [data-testid="breadcrumb-current"]`
	// trailLinkCSS is the trail's links, and trailCrumbCSS every crumb — those
	// links plus the current crumb. A separator is not a crumb.
	trailLinkCSS  = "a"
	trailCrumbCSS = `a, [aria-current="page"], [data-testid="breadcrumb-current"]`
)

// registerBreadcrumbSteps binds the trail's vocabulary: the crumbs it reads in
// order, the links among them, the crumb that names the page itself, and where
// a named link points.
func registerBreadcrumbSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^(`+selectorPattern+`) holds a "([^"]*)" link to "([^"]*)"$`, assertHoldsLinkTo)
	suite.Step(`^(`+selectorPattern+`)'s current crumb has text "([^"]*)"$`,
		assertCurrentCrumbText)
	suite.Step(`^(`+selectorPattern+`)'s current crumb matches (.+)$`, assertCurrentCrumbMatches)
	suite.Step(`^(`+selectorPattern+`)'s crumbs read ("[^"]*"(?:, "[^"]*")*)$`, assertCrumbsRead)
	suite.Step(`^(`+selectorPattern+`)'s links read ("[^"]*"(?:, "[^"]*")*)$`, assertLinksRead)
}

// assertHoldsLinkTo holds an element to carrying a link that reads the step's
// text AND points where the step says: a trail whose "Sessions" crumb leads
// somewhere else is not a way back.
func assertHoldsLinkTo(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	want := linkReading(args[1], args[2])

	got, matched, err := await(readLinks(locator.Locator(trailLinkCSS)), holdsLine(want))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s holds the links %s, want one of them %s",
			sel, summariseLines(got), want)
	}

	return nil
}

// assertCurrentCrumbText holds the crumb naming the page itself to the step's
// text — the clause that keeps the trail's tail off a parent's name.
func assertCurrentCrumbText(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	want := args[1]

	got, matched, err := await(readInnerText(locator.Locator(currentCrumbCSS)), equals(want))
	if err != nil {
		return state.fail("%s's current crumb: %w", sel, err)
	}

	if !matched {
		return state.fail("%s's current crumb reads %q, want %q", sel, got, want)
	}

	return nil
}

// assertCurrentCrumbMatches is that clause in its regexp form, which runs
// undelimited to the end of the line.
func assertCurrentCrumbMatches(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	pattern, err := regexp.Compile(args[1])
	if err != nil {
		return state.fail("the step's pattern %q does not compile: %w", args[1], err)
	}

	got, matched, err := await(readInnerText(locator.Locator(currentCrumbCSS)),
		pattern.MatchString)
	if err != nil {
		return state.fail("%s's current crumb: %w", sel, err)
	}

	if !matched {
		return state.fail("%s's current crumb reads %q, which does not match %s",
			sel, got, pattern)
	}

	return nil
}

// assertCrumbsRead holds the trail to the crumbs the step lists, in DOM order:
// the order IS the clause, so a set comparison would not serve.
func assertCrumbsRead(state *State, args []string) error {
	return assertTrailReads(state, args[0], trailCrumbCSS, "crumbs", args[1])
}

// assertLinksRead is the same clause over the LINKS alone — what a page whose
// current crumb is not a link is written with.
func assertLinksRead(state *State, args []string) error {
	return assertTrailReads(state, args[0], trailLinkCSS, "links", args[1])
}

// assertTrailReads is the shared poll; kind names which reading it took, so a
// failure says which one did not hold.
func assertTrailReads(state *State, text, css, kind, want string) error {
	sel, locator, err := locateStep(state, text)
	if err != nil {
		return err
	}

	got, matched, err := await(readQuotedTexts(locator.Locator(css)), equals(want))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s's %s read %s, want %s", sel, kind, got, want)
	}

	return nil
}

// linkReading renders one link the way readLinks renders the page's own, so the
// wanted link and the reading are compared and printed in one vocabulary.
func linkReading(text, target string) string {
	return strconv.Quote(text) + " -> " + strconv.Quote(target)
}

// holdsLine accepts a reading carrying the wanted entry as a WHOLE line: a
// substring test would accept a link to "/sessions" for one to "/".
func holdsLine(want string) func(string) bool {
	return func(value string) bool {
		return slices.Contains(strings.Split(value, "\n"), want)
	}
}

// summariseLines renders a per-line reading for a failure, or names the absence
// when the element held nothing to read.
func summariseLines(reading string) string {
	if reading == "" {
		return noneWord
	}

	return strings.Join(strings.Split(reading, "\n"), ", ")
}
