package steps

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// sidebarTestID is the panel an entry-count clause counts inside, rather
	// than page-wide: the same word renders in the breadcrumb and in the card.
	sidebarTestID = "sidebar"
	// labelColon is the punctuation the design gives a group label.
	labelColon = ":"
	// entryCountFunc counts the LEAF elements under one ancestor reading
	// exactly the label: an entry is what renders the word, so the rows and
	// panels carrying it through their children are not further entries.
	entryCountFunc = `((el, label) => String(
		Array.from(el.querySelectorAll("*")).filter(node =>
			node.children.length === 0 &&
			(node.textContent || "").trim().replace(/\s+/g, " ") === label).length))`
)

// registerSidebarEntrySteps binds the clauses about what the sidebar lists:
// how many entries read one word, and the punctuation and rule its labels
// carry.
func registerSidebarEntrySteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the sidebar renders exactly (\d+) "([^"]*)" entr(?:y|ies)$`,
		assertSidebarEntryCount)
	suite.Step(`^every ([a-z][a-z0-9-]*) ends in a colon$`, assertEveryEndsInColon)
	suite.Step(`^every ([a-z][a-z0-9-]*) is underlined$`, assertEveryUnderlined)
	registerSidebarBandSteps(suite)
}

// assertSidebarEntryCount holds the sidebar to listing a thing once: a tree
// rendering its document twice reads as two entries to a reader, and here.
func assertSidebarEntryCount(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	want, label := args[0], args[1]
	sidebar := page.Locator(elementCSS(sidebarTestID, "", "")).First()
	read := readProbe(sidebar, fmt.Sprintf(`el => %s(el, %q)`, entryCountFunc, label))

	got, matched, err := await(read, equals(want))
	if err != nil {
		return state.fail("the sidebar's %q entries: %w", label, err)
	}

	if !matched {
		return state.fail("the sidebar renders %s entries reading %q, want exactly %s",
			got, label, want)
	}

	return nil
}

// assertEveryEndsInColon holds every element of one kind to the trailing colon
// the design gives a group label.
func assertEveryEndsInColon(state *State, args []string) error {
	name := args[0]

	probe := fmt.Sprintf(`els => els.map(el => {
		const text = (el.textContent || "").trim()
		return text.endsWith(%[1]q) ? %[2]q : "a label reads " + JSON.stringify(text)
	})`, labelColon, verdictOK)

	return assertEveryElement(state, elementCSS(name, "", ""), probe,
		"every "+name+" must end in a colon")
}

// assertEveryUnderlined holds every element of one kind to carrying a line
// under its text, through the probe the single-element clause already reads.
func assertEveryUnderlined(state *State, args []string) error {
	name := args[0]

	probe := fmt.Sprintf(`els => els.map(el => {
		const read = (%[1]s)(el)
		return read === %[2]q ? %[3]q : "a label renders " + read
	})`, underlineProbe(), underlined, verdictOK)

	return assertEveryElement(state, elementCSS(name, "", ""), probe,
		"every "+name+" must be underlined")
}
