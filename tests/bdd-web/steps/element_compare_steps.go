package steps

import (
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// registerElementCompareSteps binds the clause that holds one rendered element
// against ANOTHER one's text rather than against a literal the scenario repeats.
func registerElementCompareSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^(`+selectorPattern+`) contains the text of (`+selectorPattern+`)$`,
		assertContainsElementText)
}

// assertContainsElementText holds the first element's text to carrying the
// second's: what the reader was offered is what the run must have been told.
func assertContainsElementText(state *State, args []string) error {
	sourceSel, sourceLocator, err := locateStep(state, args[1])
	if err != nil {
		return err
	}

	want, err := readInnerText(sourceLocator)()
	if err != nil {
		return state.fail("%s: %w", sourceSel, err)
	}

	if want == "" {
		return state.fail("%s reads empty, so %s would carry it vacuously",
			sourceSel, args[0])
	}

	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	got, matched, err := await(readInnerText(locator),
		func(value string) bool { return strings.Contains(value, want) })
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s reads %q, which does not carry the text of %s, %q",
			sel, got, sourceSel, want)
	}

	return nil
}
