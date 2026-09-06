package steps

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoNewScenario is returned when a clause is about the scenario a When typed
// and no When typed one.
var ErrNoNewScenario = errors.New("no step typed a uniquely-named scenario")

const (
	// sidebarGroupTestID is the outline's group element and groupAttribute is
	// where it renders the name it reads under.
	sidebarGroupTestID = "sidebar-group"
	groupAttribute     = "data-group"
	// scenariosNode is the registry's top-level node, one entry per scenario,
	// and scenarioKey the attribute a scenario row carries its id in.
	scenariosNode = "scenarios"
	scenarioKey   = "scenario-id"
)

// registerProductOutlineSteps binds the outline's own vocabulary: what the groups
// read, how the rows of one kind hang, a scenario typed into the registry, and a
// count taken over a testid PREFIX, which the selector grammar cannot express.
func registerProductOutlineSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the sidebar groups read ("[^"]*"(?:, "[^"]*")*)$`, assertSidebarGroupsRead)
	suite.Step(`^every ([a-z][a-z0-9-]*) row shares one parent$`, assertRowsShareOneParent)
	suite.Step(`^the page shows exactly (\d+) elements whose testid begins "([^"]*)"$`,
		assertPrefixedElementCount)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) types a uniquely-named `+
			`scenario into the editor$`,
		typeUniqueScenario)
	suite.Step(`^the page shows the new scenario's ([a-z][a-z0-9-]*)$`, assertNewScenarioShown)
	registerArchitectureShapeSteps(suite)
}

// assertSidebarGroupsRead holds the outline's groups to the names the step lists,
// in DOM order: the order IS the clause, so a set comparison would not serve.
func assertSidebarGroupsRead(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	want := args[0]

	got, matched, err := await(readSidebarGroupNames(page), equals(want))
	if err != nil {
		return state.fail("reading the sidebar groups: %w", err)
	}

	if !matched {
		return state.fail("the sidebar groups read %s, want %s", got, want)
	}

	return nil
}

// readSidebarGroupNames renders the groups the way the step writes them, so the
// poll and the failure speak one vocabulary.
func readSidebarGroupNames(page playwright.Page) func() (string, error) {
	script := fmt.Sprintf(`els => els.map(el => el.getAttribute(%q) || "")`, groupAttribute)

	return func() (string, error) {
		raw, err := page.Locator(elementCSS(sidebarGroupTestID, "", "")).EvaluateAll(script)
		if err != nil {
			return "", fmt.Errorf("read the sidebar groups: %w", err)
		}

		values, ok := raw.([]any)
		if !ok {
			return "", fmt.Errorf("%w: %v", ErrUnreadableProbe, raw)
		}

		names := make([]string, 0, len(values))

		for _, value := range values {
			name, isText := value.(string)
			if !isText {
				return "", fmt.Errorf("%w: %v", ErrUnreadableProbe, value)
			}

			names = append(names, strconv.Quote(name))
		}

		return strings.Join(names, ", "), nil
	}
}

// assertRowsShareOneParent holds every row of one kind to hanging off ONE element:
// rows that are siblings are flat, and rows under several parents are nested.
func assertRowsShareOneParent(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	kind := args[0] + "-row"

	got, matched, err := await(readRowParents(page, kind), sharesOneParent)
	if err != nil {
		return state.fail("reading the %s elements: %w", kind, err)
	}

	if !matched {
		return state.fail("the %s elements read %s, want rows hanging off exactly one parent",
			kind, got)
	}

	return nil
}

// readRowParents reads how many rows of one kind the page shows and how many
// elements they hang off; both stay in the reading, so a failure names which held.
func readRowParents(page playwright.Page, kind string) func() (string, error) {
	script := fmt.Sprintf(`() => {
		const rows = Array.from(document.querySelectorAll('[data-testid=%q]'))
		const parents = new Set(rows.map(row => row.parentElement))
		return "rows=" + rows.length + " parents=" + parents.size
	}`, kind)

	return func() (string, error) { return probeString(page, script) }
}

// sharesOneParent accepts a reading only once there are rows and they hang off
// one element.
func sharesOneParent(value string) bool {
	return !strings.HasPrefix(value, "rows=0 ") && strings.HasSuffix(value, "parents=1")
}

// assertPrefixedElementCount counts by a testid PREFIX — the shape of a clause
// that forbids a whole family of elements rather than one named element.
func assertPrefixedElementCount(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	want, prefix := args[0], args[1]
	locator := page.Locator(fmt.Sprintf("[data-testid^=%q]", prefix))

	got, matched, err := await(readLocatorCount(locator), equals(want))
	if err != nil {
		return state.fail("counting the elements whose testid begins %q: %w", prefix, err)
	}

	if !matched {
		return state.fail("the page shows %s elements whose testid begins %q, want exactly %s",
			got, prefix, want)
	}

	return nil
}

// readLocatorCount is readCount for a locator built here: readCount names the
// step's selector in its failure, and a testid prefix is not one.
func readLocatorCount(locator playwright.Locator) func() (string, error) {
	return func() (string, error) {
		count, err := locator.Count()
		if err != nil {
			return "", fmt.Errorf("count the matching elements: %w", err)
		}

		return strconv.Itoa(count), nil
	}
}

// typeUniqueScenario declares one more scenario by copying the registry's first
// entry and renaming the copy, so the new entry is shaped like the document it
// lands in rather than like a guess at its schema.
func typeUniqueScenario(state *State, _ []string) error {
	err := rememberAllowedDocuments(state)
	if err != nil {
		return err
	}

	raw, err := currentBuffer(state)
	if err != nil {
		return err
	}

	scenarioID := fmt.Sprintf("E2E-TBDD-%d", time.Now().UnixNano())

	edited, err := withCopiedEntry(raw, scenariosNode, scenarioID)
	if err != nil {
		return state.fail("typing a scenario under %q in the editor: %w", scenariosNode, err)
	}

	err = writeEditorBuffer(state, edited)
	if err != nil {
		return err
	}

	state.NewScenarioID = scenarioID

	return nil
}

// assertNewScenarioShown holds the outline to showing a row for the scenario the
// When typed, keyed by the id it alone carries.
func assertNewScenarioShown(state *State, args []string) error {
	if state.NewScenarioID == "" {
		return state.fail("%w", ErrNoNewScenario)
	}

	return assertElementShown(state,
		[]string{fmt.Sprintf("%s[%s=%s]", args[0], scenarioKey, state.NewScenarioID)})
}
