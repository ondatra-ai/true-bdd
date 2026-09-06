package steps

import (
	"fmt"
	"strconv"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// linksPart is what a "holds at least" clause names when it means the
	// element's own anchors rather than a testid.
	linksPart = "links"
	// breadcrumbTestID is the trail the self-link clause is about, which that
	// clause names in words rather than in the selector grammar.
	breadcrumbTestID = "content-breadcrumb"
)

// registerBreadcrumbShapeSteps binds the trail's remaining clauses: how many
// parts it holds, the space around its separators, what its current crumb is
// painted and weighted from, and the self-link it must not carry.
func registerBreadcrumbShapeSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^(`+selectorPattern+`) holds at least (\d+) (links|[a-z][a-z0-9-]*)$`,
		assertHoldsAtLeast)
	suite.Step(`^every ([a-z][a-z0-9-]*) has at least (\d+) pixels of space `+
		`on both sides$`, assertSpacedBothSides)
	suite.Step(`^(`+selectorPattern+`)'s current crumb resolves "([^"]*)" `+
		`from the token "([^"]*)"$`, assertCurrentCrumbToken)
	suite.Step(`^(`+selectorPattern+`)'s current crumb is (bold|not bold)$`,
		assertCurrentCrumbWeight)
	suite.Step(`^no breadcrumb link points at the current route$`,
		refuteBreadcrumbSelfLink)
	registerHoverSteps(suite)
}

// assertHoldsAtLeast holds an element to carrying at least as many of a part as
// the clause names: "links" is its own anchors, any other word is a testid.
func assertHoldsAtLeast(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	want, err := strconv.Atoi(args[1])
	if err != nil {
		return state.fail("the step's count %q does not parse: %w", args[1], err)
	}

	part := args[2]

	got, matched, err := await(readMatchCount(locator.Locator(partCSS(part))),
		atLeast(want))
	if err != nil {
		return state.fail("%s's %s: %w", sel, part, err)
	}

	if !matched {
		return state.fail("%s holds %s %s, want at least %d", sel, got, part, want)
	}

	return nil
}

// partCSS is where a clause's part renders: an element's own anchors, or any
// other word as the testid of that name.
func partCSS(part string) string {
	if part == linksPart {
		return trailLinkCSS
	}

	return elementCSS(part, "", "")
}

// readMatchCount reads how many elements a locator matches, rendered as the
// digits the step writes so the poll and the failure speak one vocabulary.
func readMatchCount(locator playwright.Locator) func() (string, error) {
	return func() (string, error) {
		count, err := locator.Count()
		if err != nil {
			return "", fmt.Errorf("count the matching elements: %w", err)
		}

		return strconv.Itoa(count), nil
	}
}

// atLeast accepts a counted reading no smaller than the clause's floor.
func atLeast(want int) func(string) bool {
	return func(value string) bool {
		count, err := strconv.Atoi(value)

		return err == nil && count >= want
	}
}

// assertSpacedBothSides holds every element of one kind to the space the design
// gives it on both sides, measured between boxes: padding, margin and a flex
// gap are one thing to a reader.
func assertSpacedBothSides(state *State, args []string) error {
	name := args[0]

	want, err := pixels(state, args[1])
	if err != nil {
		return err
	}

	probe := fmt.Sprintf(`els => els.map(el => {
		const box = el.getBoundingClientRect()
		const before = el.previousElementSibling
		const after = el.nextElementSibling
		if (!before || !after) { return "a %[1]s has no neighbour on one side" }
		const lead = box.left - before.getBoundingClientRect().right
		const trail = after.getBoundingClientRect().left - box.right
		const gap = Math.min(lead, trail)
		return gap >= %[2]f ? %[3]q : "a %[1]s has " + gap.toFixed(1) + " pixels of space"
	})`, name, want-lengthSlack, verdictOK)

	return assertEveryElement(state, elementCSS(name, "", ""), probe,
		fmt.Sprintf("every %s must have at least %.0f pixels of space on both sides",
			name, want))
}

// assertCurrentCrumbToken holds the crumb naming the page itself to painting
// one property from its token, through the comparison every token clause
// shares.
func assertCurrentCrumbToken(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	crumb := locator.Locator(currentCrumbCSS).First()
	property, token := args[1], args[2]

	reading, matched, err := await(readTokenReading(crumb, property, token),
		readingsAgree)
	if err != nil {
		return state.fail("%s's current crumb: %w", sel, err)
	}

	if !matched {
		got, want := renderReading(reading)

		return state.fail("%s's current crumb has %s = %s, want the value of %s, "+
			"which is %s", sel, property, got, token, want)
	}

	return nil
}

// assertCurrentCrumbWeight is the weight clause on that crumb, in either
// polarity, against the same bold floor every other weight clause reads.
func assertCurrentCrumbWeight(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	crumb := locator.Locator(currentCrumbCSS).First()
	polarity := args[1]

	got, matched, err := await(readComputedStyle(crumb, fontWeightProperty),
		weighsAs(polarity))
	if err != nil {
		return state.fail("%s's current crumb: %w", sel, err)
	}

	if !matched {
		return state.fail("%s's current crumb has %s = %q, want it %s "+
			"(bold is %d or more)", sel, fontWeightProperty, got, polarity, boldFloor)
	}

	return nil
}

// refuteBreadcrumbSelfLink holds the trail to linking only its parents: a crumb
// pointing at the page it sits on is not a way anywhere. A trailing slash is
// not a difference a reader can click.
func refuteBreadcrumbSelfLink(state *State, _ []string) error {
	probe := fmt.Sprintf(`els => els.map(el => {
		const here = location.pathname.replace(/\/$/, "")
		const there = new URL(el.href, location.href).pathname.replace(/\/$/, "")
		return there === here ? "a crumb links to " + there : %[1]q
	})`, verdictOK)

	return assertEveryElement(state,
		elementCSS(breadcrumbTestID, "", "")+" "+trailLinkCSS, probe,
		"no breadcrumb link may point at the current route")
}
