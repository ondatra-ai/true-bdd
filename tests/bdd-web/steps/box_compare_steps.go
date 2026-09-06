package steps

import (
	"math"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// registerBoxCompareSteps binds the clauses comparing one element's box against
// another's: which is wider, where one begins against the other's left edge,
// and whether a cover is as wide and as tall as what it hides.
func registerBoxCompareSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^(`+selectorPattern+`) is wider than (`+selectorPattern+`)$`,
		assertWiderThan)
	suite.Step(`^(`+selectorPattern+`) begins at (`+selectorPattern+`)'s left edge, `+
		`within (\d+) pixels?$`, assertBeginsAtLeftEdge)
	suite.Step(`^(`+selectorPattern+`) is as wide as (`+selectorPattern+`), `+
		`within (\d+) pixels?$`, assertAsWideAs)
	suite.Step(`^(`+selectorPattern+`) is at least as tall as (`+selectorPattern+`)$`,
		assertAtLeastAsTall)
}

// assertWiderThan holds one pane to being wider than another, past the slack
// that keeps sub-pixel rounding from being a difference.
func assertWiderThan(state *State, args []string) error {
	sel, box, err := currentBox(state, args[0])
	if err != nil {
		return err
	}

	other, against, err := currentBox(state, args[1])
	if err != nil {
		return err
	}

	if box.Width > against.Width+edgeSlack {
		return nil
	}

	return state.fail("%s is %.1f pixels wide and %s is %.1f, want it wider",
		sel, box.Width, other, against.Width)
}

// assertBeginsAtLeftEdge holds one element to starting where another does,
// within the slack the clause allows: a cover offset by a sliver leaves the
// sliver showing.
func assertBeginsAtLeftEdge(state *State, args []string) error {
	sel, box, other, against, tolerance, err := comparedBoxes(state, args)
	if err != nil {
		return err
	}

	if math.Abs(box.X-against.X) <= tolerance {
		return nil
	}

	return state.fail("%s begins at %.1f and %s's left edge is at %.1f, "+
		"want them within %.0f pixels", sel, box.X, other, against.X, tolerance)
}

// assertAsWideAs holds two elements to one width, within the slack the clause
// allows.
func assertAsWideAs(state *State, args []string) error {
	sel, box, other, against, tolerance, err := comparedBoxes(state, args)
	if err != nil {
		return err
	}

	if math.Abs(box.Width-against.Width) <= tolerance {
		return nil
	}

	return state.fail("%s is %.1f pixels wide and %s is %.1f, want them within "+
		"%.0f pixels", sel, box.Width, other, against.Width, tolerance)
}

// assertAtLeastAsTall holds one element to covering another down its whole
// height, which is what leaves no strip of the hidden panel showing.
func assertAtLeastAsTall(state *State, args []string) error {
	sel, box, err := currentBox(state, args[0])
	if err != nil {
		return err
	}

	other, against, err := currentBox(state, args[1])
	if err != nil {
		return err
	}

	if box.Height >= against.Height-edgeSlack {
		return nil
	}

	return state.fail("%s is %.1f pixels tall and %s is %.1f, want it at least "+
		"as tall", sel, box.Height, other, against.Height)
}

// comparedBoxes measures both of a comparison clause's elements and reads the
// slack it allows — the three things every clause above opens with.
func comparedBoxes(state *State, args []string) (
	selector, elementBox, selector, elementBox, float64, error,
) {
	sel, box, err := currentBox(state, args[0])
	if err != nil {
		return sel, box, selector{}, elementBox{}, 0, err
	}

	other, against, err := currentBox(state, args[1])
	if err != nil {
		return sel, box, other, against, 0, err
	}

	tolerance, err := pixels(state, args[2])

	return sel, box, other, against, tolerance, err
}
