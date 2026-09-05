package steps

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoTableRows is returned when a clause about every row finds no rows at all:
// a vacuous pass is exactly what it refuses.
var ErrNoTableRows = errors.New("the page shows no registry rows")

// ErrRowDisagreesWithRegistry is returned when a row says something the entry it
// stands for does not.
var ErrRowDisagreesWithRegistry = errors.New("a row disagrees with the registry")

const (
	// scenarioRowTestID is the registry table's row, keyed by the entry it
	// stands for; the page renders one per entry of the registry.
	scenarioRowTestID = "scenario-table-row"
	// columnHeaderCSS is where a table names its columns, in either shape the
	// design system may render it in.
	columnHeaderCSS = `th, [role="columnheader"], [data-testid="scenario-table-column"]`
	// serviceField is the scalar a registry entry names its service in.
	serviceField = "service"
	// rowFieldCount is how many readings the row probe answers per row.
	rowFieldCount = 3
)

// rowReading is one registry row as the probe answers it: the entry it stands
// for, its rendered text, and where its links point.
type rowReading struct {
	ID    string
	Text  string
	Links string
}

// registerScenarioTableSteps binds the registry table: what its columns read,
// that it holds one row per entry, and what each row says about the entry it
// stands for.
func registerScenarioTableSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^(`+selectorPattern+`)'s columns read ("[^"]*"(?:, "[^"]*")*)$`,
		assertColumnsRead)
	suite.Step(`^the page shows one (`+selectorPattern+`) per entry of "([^"]+)"$`,
		assertOneElementPerEntry)
	suite.Step(`^every row's ([a-z][a-z0-9-]*) is a link carrying that row's id$`,
		assertEveryRowIDLink)
	suite.Step(`^every row's description and service cells hold the registry's values$`,
		assertEveryRowHoldsRegistryValues)
	suite.Step(`^every row with a linked story links to it by the story's own id$`,
		assertLinkedStoryRows)
	suite.Step(`^every row without a linked story shows no linked-story link$`,
		assertUnlinkedStoryRows)
}

// assertColumnsRead holds the table's column headers to the names the step
// lists, in order. Case-blind: which case a header is typeset in is the design
// system's business, which words it reads is the clause.
func assertColumnsRead(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	want := args[1]

	got, matched, err := await(readQuotedTexts(locator.Locator(columnHeaderCSS)),
		func(value string) bool { return strings.EqualFold(value, want) })
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s's columns read %s, want %s", sel, got, want)
	}

	return nil
}

// assertOneElementPerEntry holds how many rows the page shows to how many
// entries the registry declares, read from the document rather than from a
// number the scenario repeats.
func assertOneElementPerEntry(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	sel, err := parseSelector(args[0])
	if err != nil {
		return state.fail("%w", err)
	}

	relPath := args[1]

	ids, err := registryEntryIDs(state, relPath)
	if err != nil {
		return err
	}

	want := strconv.Itoa(len(ids))

	got, matched, err := await(readCount(page, sel), equals(want))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("the page shows %s %s, want one per entry of %s, which declares %s",
			got, sel, relPath, want)
	}

	return nil
}

// registryEntryIDs are the ids one registry declares, in document order.
func registryEntryIDs(state *State, relPath string) ([]string, error) {
	raw, err := fixtureFile(state, relPath)
	if err != nil {
		return nil, err
	}

	ids, err := entryIDs(raw)
	if err != nil {
		return nil, state.fail("%s: %w", relPath, err)
	}

	return ids, nil
}

// entryIDs are the keys one level under a registry's `scenarios:` node — the
// entries themselves, and none of the fields they carry.
func entryIDs(raw string) ([]string, error) {
	lines := strings.Split(raw, "\n")

	first, err := firstEntryUnder(lines, scenariosNode)
	if err != nil {
		return nil, err
	}

	indent := indentOf(lines[first])
	// Compiled per call: a package-level regexp is a global.
	key := regexp.MustCompile(`^` + indent + `([^\s#:]+):\s*(?:#.*)?$`)
	ids := []string{}

	for _, line := range lines[first:] {
		if strings.TrimSpace(line) != "" && len(indentOf(line)) < len(indent) {
			break
		}

		match := key.FindStringSubmatch(line)
		if match != nil {
			ids = append(ids, match[1])
		}
	}

	return ids, nil
}

// assertEveryRowIDLink holds every row's named cell to being a link that carries
// that row's own id — one shared link reads correctly and leads everywhere.
func assertEveryRowIDLink(state *State, args []string) error {
	childTestID := args[0]

	page, err := state.page()
	if err != nil {
		return err
	}

	got, matched, err := await(readRowLinkCells(page, childTestID), everyRowLinksItsID)
	if err != nil {
		return state.fail("reading each row's %s: %w", childTestID, err)
	}

	if !matched {
		return state.fail("the rows' %s read %s, want each a link carrying its own row's id",
			childTestID, summariseLines(got))
	}

	return nil
}

// everyRowLinksItsID answers whether every row's named cell is an anchor whose
// text carries the row's own id.
func everyRowLinksItsID(reading string) bool {
	if reading == "" {
		return false
	}

	for _, line := range strings.Split(reading, "\n") {
		parts := strings.Split(line, linkFieldSeparator)
		if len(parts) != rowFieldCount {
			return false
		}

		if parts[0] == "" || parts[2] == "" || !strings.Contains(parts[1], parts[0]) {
			return false
		}
	}

	return true
}

// readRowLinkCells answers with one line per row: the id it carries, and the
// text and target of the child the clause names.
func readRowLinkCells(page playwright.Page, childTestID string) func() (string, error) {
	script := fmt.Sprintf(`() => Array.from(document.querySelectorAll('[data-testid=%q]')).map(row => {
		const cell = row.querySelector('[data-testid=%q]')
		const link = cell && cell.tagName === "A" ? cell : (cell ? cell.querySelector("a[href]") : null)
		return [
			row.getAttribute(%q) || "",
			cell ? (cell.textContent || "").trim().replace(/\s+/g, " ") : "",
			link ? (link.getAttribute("href") || "") : ""
		].join(" ")
	}).join("\n")`, scenarioRowTestID, childTestID, domAttribute(scenarioKey))

	return func() (string, error) { return probeString(page, script) }
}

// assertEveryRowHoldsRegistryValues holds each row to carrying the description
// and service the registry states for the entry it stands for.
func assertEveryRowHoldsRegistryValues(state *State, _ []string) error {
	rows, err := tableRows(state)
	if err != nil {
		return err
	}

	for _, row := range rows {
		err = assertRowMatchesEntry(state, row)
		if err != nil {
			return err
		}
	}

	return nil
}

// assertRowMatchesEntry holds one row to its own entry's scalars.
func assertRowMatchesEntry(state *State, row rowReading) error {
	for _, field := range []string{descriptionField, serviceField} {
		want, err := entryField(state, row.ID, field)
		if err != nil {
			return err
		}

		if !strings.Contains(row.Text, want) {
			return state.fail("%w: row %s reads %q, which does not carry the %s of its "+
				"entry, %q", ErrRowDisagreesWithRegistry, row.ID, row.Text, field, want)
		}
	}

	return nil
}

// assertLinkedStoryRows holds every row whose entry links a story to carrying a
// link to THAT story, by the story's own id.
func assertLinkedStoryRows(state *State, _ []string) error {
	return assertStoryLinkage(state, true)
}

// assertUnlinkedStoryRows is the twin: a row whose entry links no story offers
// no story link at all, rather than one pointing nowhere.
func assertUnlinkedStoryRows(state *State, _ []string) error {
	return assertStoryLinkage(state, false)
}

// assertStoryLinkage grades the rows the clause is about; wantLinked names which
// half of the registry those are.
func assertStoryLinkage(state *State, wantLinked bool) error {
	rows, err := tableRows(state)
	if err != nil {
		return err
	}

	for _, row := range rows {
		stories, storiesErr := linkedStoryIDs(state, row.ID)
		if storiesErr != nil {
			return storiesErr
		}

		if (len(stories) > 0) != wantLinked {
			continue
		}

		err = assertRowStoryLinks(state, row, stories)
		if err != nil {
			return err
		}
	}

	return nil
}

// assertRowStoryLinks holds one row to linking each story its entry names, and a
// row whose entry names none to linking no story at all.
func assertRowStoryLinks(state *State, row rowReading, stories []string) error {
	if len(stories) == 0 {
		if strings.Contains(row.Links, storyRoute) {
			return state.fail("%w: row %s links %q, want no story link: its entry names none",
				ErrRowDisagreesWithRegistry, row.ID, row.Links)
		}

		return nil
	}

	for _, storyID := range stories {
		if !strings.Contains(row.Links, storyRoute+storyID) {
			return state.fail("%w: row %s links %q, want a link to %s: that is the story its "+
				"entry names", ErrRowDisagreesWithRegistry, row.ID, row.Links,
				storyRoute+storyID)
		}
	}

	return nil
}

// linkedStoryIDs are the stories one registry entry names, by the id each
// story's own filename carries.
func linkedStoryIDs(state *State, scenarioID string) ([]string, error) {
	raw, err := fixtureFile(state, registryDocument)
	if err != nil {
		return nil, err
	}

	block, err := entryBlock(raw, scenarioID)
	if err != nil {
		return nil, state.fail("%s: %w", registryDocument, err)
	}

	// Compiled per call: a package-level regexp is a global.
	reference := regexp.MustCompile(`(?m)^\s*-\s+story:\s*["']?([^"'\s]+)["']?\s*$`)
	ids := []string{}

	for _, match := range reference.FindAllStringSubmatch(block, -1) {
		storyID, _, _ := strings.Cut(strings.TrimSuffix(filepath.Base(match[1]), ".yaml"), "-")
		ids = append(ids, storyID)
	}

	return ids, nil
}

// tableRows is the rows the page shows now, and says so when it shows none.
func tableRows(state *State) ([]rowReading, error) {
	page, err := state.page()
	if err != nil {
		return nil, err
	}

	got, matched, err := await(readTableRows(page),
		func(value string) bool { return value != "" })
	if err != nil {
		return nil, state.fail("reading the registry rows: %w", err)
	}

	if !matched {
		return nil, state.fail("%w", ErrNoTableRows)
	}

	rows := []rowReading{}

	for _, line := range strings.Split(got, "\n") {
		parts := strings.Split(line, linkFieldSeparator)
		if len(parts) != rowFieldCount {
			return nil, state.fail("%w: a row read as %q", ErrUnreadableProbe, line)
		}

		rows = append(rows, rowReading{ID: parts[0], Text: parts[1], Links: parts[2]})
	}

	return rows, nil
}

// readTableRows answers with one line per row: the id it carries, its text, and
// its links' targets — one evaluation, so every row is read at one instant.
func readTableRows(page playwright.Page) func() (string, error) {
	script := fmt.Sprintf(`() => Array.from(document.querySelectorAll('[data-testid=%q]')).map(row => [
		row.getAttribute(%q) || "",
		(row.textContent || "").trim().replace(/\s+/g, " "),
		Array.from(row.querySelectorAll("a[href]")).map(link => link.getAttribute("href")).join(" ")
	].join(" ")).join("\n")`, scenarioRowTestID, domAttribute(scenarioKey))

	return func() (string, error) { return probeString(page, script) }
}
