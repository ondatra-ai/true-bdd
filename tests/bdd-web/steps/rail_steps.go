package steps

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// railItemCSS, railIconCSS and railLabelCSS are the rail's testid contract,
	// the same one `rail-item[section=…]` resolves through in selector.go.
	railItemCSS  = `[data-testid="rail-item"]`
	railIconCSS  = `[data-testid="rail-icon"]`
	railLabelCSS = `[data-testid="rail-label"]`
	// verdictOK is what a per-element probe answers for an element satisfying the
	// clause; anything else IS the failure the reader is shown.
	verdictOK = "ok"
	// letterSpacingProperty is the property a letter-spacing token is read on.
	letterSpacingProperty = "letter-spacing"
	// sessionsListRoute is where the way back to the sessions list points — the
	// target E2E-186's trail already names.
	sessionsListRoute = "/"
	// glyphSlack absorbs sub-pixel rounding when asking whether a label is
	// clipped or an icon leads its label.
	glyphSlack = 1
	// lowerPartFrom is where an element's lower part begins, as a fraction of
	// its own height.
	lowerPartFrom = 0.5
)

// registerRailSteps binds the section rail's vocabulary: what every entry on it
// renders, how its labels are set, and where the way back to the sessions list
// sits on it.
func registerRailSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^every rail item shows (an icon glyph|its full label)$`, assertRailItemsShow)
	suite.Step(`^every rail label has computed "([^"]*)" = "([^"]*)"$`, assertRailLabelsComputed)
	suite.Step(`^every rail label is letter-spaced from the token "([^"]*)"$`,
		assertRailLabelsLetterSpaced)
	suite.Step(`^every rail icon sits above or before its label$`, assertRailIconsLead)
	suite.Step(`^(`+selectorPattern+`) holds a sessions link$`, assertHoldsSessionsLink)
	suite.Step(`^(`+selectorPattern+`) sits in the lower part of (`+selectorPattern+`)$`,
		assertSitsInLowerPart)
}

// assertRailItemsShow holds every rail entry to rendering the part the clause
// names: the glyph that leads it, or the whole word beside it.
func assertRailItemsShow(state *State, args []string) error {
	return assertEveryElement(state, railItemCSS, railItemProbe(args[0]),
		"every rail item must show "+args[0])
}

// railItemProbe is the per-item probe the two wordings pick between. A label is
// "full" when it is neither empty nor clipped by its own box.
func railItemProbe(shown string) string {
	if shown == "an icon glyph" {
		return fmt.Sprintf(`els => els.map(el => {
			const icon = el.querySelector(%[1]q)
			if (!icon) { return "an item renders no rail icon" }
			const glyph = (icon.textContent || "").trim()
			return glyph === "" ? "an item's icon is empty" : %[2]q
		})`, railIconCSS, verdictOK)
	}

	return fmt.Sprintf(`els => els.map(el => {
		const label = el.querySelector(%[1]q)
		if (!label) { return "an item renders no rail label" }
		const text = (label.textContent || "").trim()
		if (text === "") { return "an item's label is empty" }
		if (label.scrollWidth > label.clientWidth + %[2]d) {
			return "the label " + text + " is clipped"
		}
		return %[3]q
	})`, railLabelCSS, glyphSlack, verdictOK)
}

// assertRailLabelsComputed holds every rail label to one computed property.
func assertRailLabelsComputed(state *State, args []string) error {
	property, want := args[0], args[1]

	probe := fmt.Sprintf(`els => els.map(el => {
		const got = getComputedStyle(el).getPropertyValue(%[1]q).trim()
		return got === %[2]q ? %[3]q : "a label has " + %[1]q + " = " + got
	})`, property, want, verdictOK)

	return assertEveryElement(state, railLabelCSS, probe,
		fmt.Sprintf("every rail label must have computed %q = %q", property, want))
}

// assertRailLabelsLetterSpaced holds every rail label's tracking to the value
// its token resolves to, through the same probe the single-element clause uses.
func assertRailLabelsLetterSpaced(state *State, args []string) error {
	token := args[0]

	probe := fmt.Sprintf(`els => els.map(el => {
		const parts = %[1]s(el, %[2]q, %[3]q).split("\u0000")
		if (parts[1] === "") { return "the page defines no " + %[3]q }
		return parts[0] === parts[1] ? %[4]q :
			"a label is letter-spaced " + parts[0] + ", and " + %[3]q +
			" resolves to " + parts[1]
	})`, tokenReadingFunc, letterSpacingProperty, token, verdictOK)

	return assertEveryElement(state, railLabelCSS, probe,
		"every rail label must be letter-spaced from the token "+token)
}

// assertRailIconsLead holds every entry's glyph to leading its word, in either
// reading direction the rail may be laid out in.
func assertRailIconsLead(state *State, _ []string) error {
	probe := fmt.Sprintf(`els => els.map(el => {
		const icon = el.querySelector(%[1]q)
		const label = el.querySelector(%[2]q)
		if (!icon || !label) { return "an item renders no icon and label pair" }
		const glyph = icon.getBoundingClientRect()
		const word = label.getBoundingClientRect()
		if (glyph.bottom <= word.top + %[3]d) { return %[4]q }
		if (glyph.right <= word.left + %[3]d) { return %[4]q }
		return "an icon at " + Math.round(glyph.x) + "," + Math.round(glyph.y) +
			" is neither above nor before its label at " +
			Math.round(word.x) + "," + Math.round(word.y)
	})`, railIconCSS, railLabelCSS, glyphSlack, verdictOK)

	return assertEveryElement(state, railItemCSS, probe,
		"every rail icon must sit above or before its label")
}

// assertHoldsSessionsLink holds an element to carrying the way back: a link that
// reads as sessions AND points at the list itself.
func assertHoldsSessionsLink(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	// Compiled per call: a package-level regexp is a global.
	wanted := regexp.MustCompile(`(?i)^"[^"]*sessions?[^"]*" -> ` +
		regexp.QuoteMeta(strconv.Quote(sessionsListRoute)) + `$`)

	got, matched, err := await(readLinks(locator.Locator(trailLinkCSS)),
		anyLineMatches(wanted))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s holds the links %s, want one reading as sessions and "+
			"pointing at %q", sel, summariseLines(got), sessionsListRoute)
	}

	return nil
}

// assertSitsInLowerPart holds one element to sitting in the bottom half of
// another: what "pinned at the foot of the rail" means with no pixel named.
func assertSitsInLowerPart(state *State, args []string) error {
	sel, box, err := currentBox(state, args[0])
	if err != nil {
		return err
	}

	container, around, err := currentBox(state, args[1])
	if err != nil {
		return err
	}

	lower := around.Y + around.Height*lowerPartFrom

	if box.Y >= lower-edgeSlack {
		return nil
	}

	return state.fail("%s begins at %.1f and %s's lower part begins at %.1f, "+
		"want it to begin at or after that", sel, box.Y, container, lower)
}

// assertEveryElement holds every element a CSS matches to a probe's verdict,
// which is the shape all four rail clauses share.
func assertEveryElement(state *State, css, probe, wanted string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	got, matched, err := await(readVerdicts(page, css, probe), everyVerdictOK)
	if err != nil {
		return state.fail("reading whether %s: %w", wanted, err)
	}

	if !matched {
		return state.fail("%s: %s", wanted, complaints(got))
	}

	return nil
}

// readVerdicts reads one verdict per matched element, one per line, so a clause
// about EVERY element polls through the same await as every other.
func readVerdicts(page playwright.Page, css, probe string) func() (string, error) {
	return func() (string, error) {
		values, err := evaluateAllStrings(page.Locator(css), probe)
		if err != nil {
			return "", err
		}

		return strings.Join(values, "\n"), nil
	}
}

// everyVerdictOK accepts a reading in which every element answered ok, and
// refuses an empty one: a clause about every rail item is not satisfied by a
// page that renders none.
func everyVerdictOK(reading string) bool {
	if reading == "" {
		return false
	}

	for _, line := range strings.Split(reading, "\n") {
		if line != verdictOK {
			return false
		}
	}

	return true
}

// complaints renders the verdicts that were not ok, or names the absence when
// the page rendered nothing to judge.
func complaints(reading string) string {
	if reading == "" {
		return "the page shows none"
	}

	kept := make([]string, 0, len(reading))

	for _, line := range strings.Split(reading, "\n") {
		if line != verdictOK {
			kept = append(kept, line)
		}
	}

	return strings.Join(kept, "; ")
}
