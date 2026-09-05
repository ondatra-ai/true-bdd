package steps

import (
	"regexp"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// headingCSS is every heading level: which one carries the text is the page's
// business, that some heading does is the clause.
const headingCSS = "h1, h2, h3, h4, h5, h6"

// registerPageTextSteps binds the page-wide text clauses: text the page must
// carry somewhere, and the heading form of the same question. The literal-text
// clause is the "the page shows" assertion under a second wording.
func registerPageTextSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the page holds the text "([^"]*)"$`, assertText)
	suite.Step(`^the page holds text matching (.+)$`, assertPageMatches)
	suite.Step(`^the page holds a heading matching (.+)$`, assertHeadingMatches)
}

// assertPageMatches holds the page's own text to matching the step's regexp,
// which runs undelimited to the end of the line.
func assertPageMatches(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	pattern, err := regexp.Compile(args[0])
	if err != nil {
		return state.fail("the step's pattern %q does not compile: %w", args[0], err)
	}

	got, matched, err := await(readInnerText(page.Locator(bodyKey)), pattern.MatchString)
	if err != nil {
		return state.fail("reading the page's text: %w", err)
	}

	if !matched {
		return state.fail("the page reads %q, which does not match %s", got, pattern)
	}

	return nil
}

// assertHeadingMatches holds ONE of the page's headings to that regexp, read
// per heading so a pattern cannot span two of them.
func assertHeadingMatches(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	pattern, err := regexp.Compile(args[0])
	if err != nil {
		return state.fail("the step's pattern %q does not compile: %w", args[0], err)
	}

	got, matched, err := await(readTexts(page.Locator(headingCSS)), anyLineMatches(pattern))
	if err != nil {
		return state.fail("reading the page's headings: %w", err)
	}

	if !matched {
		return state.fail("the page's headings read %s, want one matching %s",
			summariseLines(got), pattern)
	}

	return nil
}

// anyLineMatches accepts a per-line reading in which one line matches.
func anyLineMatches(pattern *regexp.Regexp) func(string) bool {
	return func(value string) bool {
		for _, line := range strings.Split(value, "\n") {
			if pattern.MatchString(line) {
				return true
			}
		}

		return false
	}
}
