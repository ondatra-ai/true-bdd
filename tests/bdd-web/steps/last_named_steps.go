package steps

import (
	"errors"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoNamedElement is returned when a clause says "it" and no earlier clause
// named an element for it to mean.
var ErrNoNamedElement = errors.New(`no earlier clause named an element for "it" to mean`)

// registerLastNamedSteps binds the clauses written about "it" — the element the
// clause before them named, resolved through the one funnel every clause
// locates an element through.
func registerLastNamedSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^clicking it changes its glyph to "([^"]*)"$`, assertClickChangesGlyph)
}

// assertClickChangesGlyph clicks that element and holds its text to the glyph the
// clause names: what a caret shows IS the direction it will move in.
func assertClickChangesGlyph(state *State, args []string) error {
	if state.LastSelector == "" {
		return state.fail("%w", ErrNoNamedElement)
	}

	sel, locator, err := locateStep(state, state.LastSelector)
	if err != nil {
		return err
	}

	err = locator.Click()
	if err != nil {
		return state.fail("clicking %s: %w", sel, err)
	}

	want := args[0]

	got, matched, err := await(readInnerText(locator), equals(want))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s reads %q after the click, want the glyph %q", sel, got, want)
	}

	return nil
}
