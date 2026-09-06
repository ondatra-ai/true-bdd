package steps

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoChipsRendered is returned when a clause about the inventory chips runs
// and the list rendered none.
var ErrNoChipsRendered = errors.New("the inventory list rendered no chip")

// ErrNoChipSnapshot is returned when a clause compares the chips to where they
// first rendered and no earlier clause measured them.
var ErrNoChipSnapshot = errors.New("no earlier clause measured the chips")

// chipReading is one chip as the page rendered it: the entry it belongs to, the
// status it carries, and the box it occupies.
type chipReading struct {
	Key    string     `json:"key"`
	Status string     `json:"status"`
	Box    elementBox `json:"box"`
}

// registerOverviewChipSteps binds the clauses about the chips as data arrives:
// that the list renders before its statuses do, and that settling moves none.
func registerOverviewChipSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^at least one chip is pending when the list first renders$`,
		assertChipsStartPending)
	suite.Step(`^no chip's box moved or resized by more than (\d+) pixels$`,
		assertChipBoxesHeld)
}

// chipsProbe reads every chip's entry, status and box in ONE evaluation: a
// clause about the list's first render is about one instant, not one chip.
func chipsProbe() string {
	return fmt.Sprintf(`() => JSON.stringify(
		Array.from(document.querySelectorAll('%[1]s')).map(el => {
			const row = el.closest('%[2]s')
			const box = el.getBoundingClientRect()
			return {key: row ? (row.getAttribute("data-key") || "") : "",
				status: el.getAttribute(%[3]q) || "",
				box: {x: box.x, y: box.y, width: box.width, height: box.height}}
		}))`,
		elementCSS(inventoryChipTestID, "", ""), elementCSS(inventoryRowTestID, "", ""),
		inventoryStatusAttribute)
}

// assertChipsStartPending holds the list to rendering its chips before their
// statuses arrive, and records where they sat: the clause after it is compared
// against that first render and nothing else records it.
func assertChipsStartPending(state *State, _ []string) error {
	chips, err := awaitFirstChips(state)
	if err != nil {
		return err
	}

	state.ChipBoxes = chipBoxes(chips)

	for _, chip := range chips {
		if !knownStatus(chip.Status) {
			return nil
		}
	}

	return state.fail("the list first rendered %d chip(s) and every one already "+
		"carried a live status (%s), want at least one still pending",
		len(chips), describeChips(chips))
}

// awaitFirstChips is the first reading in which the list holds chips at all,
// which is what "when the list first renders" names.
func awaitFirstChips(state *State) ([]chipReading, error) {
	page, err := state.page()
	if err != nil {
		return nil, err
	}

	// An empty list stringifies to exactly "[]", so the reading names its own
	// emptiness without being decoded first.
	reading, matched, err := await(readChips(page),
		func(value string) bool { return value != "[]" })
	if err != nil {
		return nil, state.fail("reading the inventory chips: %w", err)
	}

	if !matched {
		return nil, state.fail("%w", ErrNoChipsRendered)
	}

	return decodeChips(state, reading)
}

// readChips reads that JSON as a reader, so the chip clauses poll through the
// same await every other value clause uses.
func readChips(page playwright.Page) func() (string, error) {
	return func() (string, error) { return probeString(page, chipsProbe()) }
}

// decodeChips is one reading as this suite keeps it.
func decodeChips(state *State, reading string) ([]chipReading, error) {
	var chips []chipReading

	err := json.Unmarshal([]byte(reading), &chips)
	if err != nil {
		return nil, state.fail("decoding the inventory chips: %w\n%s", err, reading)
	}

	return chips, nil
}

// chipBoxes is where each chip sat, keyed by the entry it belongs to: an index
// would name a different chip the moment the list renders one more.
func chipBoxes(chips []chipReading) map[string]elementBox {
	boxes := map[string]elementBox{}

	for index, chip := range chips {
		boxes[chipKey(chip, index)] = chip.Box
	}

	return boxes
}

// chipKey names one chip: its own entry, or its place in the list when the row
// carries no key.
func chipKey(chip chipReading, index int) string {
	if chip.Key != "" {
		return chip.Key
	}

	return fmt.Sprintf("chip %d", index+1)
}

// assertChipBoxesHeld holds every chip to the box it had when the list first
// rendered: data arriving may change what a chip says and nothing else. Read
// once — the clause before it already held the list to having settled.
func assertChipBoxesHeld(state *State, args []string) error {
	if state.ChipBoxes == nil {
		return state.fail("%w", ErrNoChipSnapshot)
	}

	tolerance, err := pixels(state, args[0])
	if err != nil {
		return err
	}

	page, err := state.page()
	if err != nil {
		return err
	}

	reading, err := readChips(page)()
	if err != nil {
		return state.fail("reading the inventory chips: %w", err)
	}

	chips, err := decodeChips(state, reading)
	if err != nil {
		return err
	}

	return compareChipBoxes(state, chips, tolerance)
}

// compareChipBoxes grades this reading against the first render, naming the
// first chip that moved or that the list has since added.
func compareChipBoxes(state *State, chips []chipReading, tolerance float64) error {
	for index, chip := range chips {
		key := chipKey(chip, index)

		before, measured := state.ChipBoxes[key]
		if !measured {
			return state.fail("the list now shows a chip for %s that it did not show "+
				"when it first rendered", key)
		}

		if boxHeld(before, chip.Box, tolerance) {
			continue
		}

		return state.fail("%s's chip covers %s and covered %s when the list first "+
			"rendered, want them within %.0f pixels",
			key, renderBox(chip.Box), renderBox(before), tolerance)
	}

	return missingChips(state, chips)
}

// missingChips names a chip the list has since dropped: a list rendering fewer
// chips moved the ones below it, whatever their own boxes read.
func missingChips(state *State, chips []chipReading) error {
	shown := map[string]bool{}

	for index, chip := range chips {
		shown[chipKey(chip, index)] = true
	}

	for _, key := range sortedBoxKeys(state.ChipBoxes) {
		if !shown[key] {
			return state.fail("the list showed a chip for %s when it first rendered "+
				"and shows none now", key)
		}
	}

	return nil
}

// sortedBoxKeys is those entries in a stable order, so a failure names the same
// first offender on every run.
func sortedBoxKeys(boxes map[string]elementBox) []string {
	keys := make([]string, 0, len(boxes))
	for key := range boxes {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

// describeChips renders what the chips DID carry, so a failure carries the
// alternative rather than only the absence.
func describeChips(chips []chipReading) string {
	lines := make([]string, 0, len(chips))

	for index, chip := range chips {
		lines = append(lines, chipKey(chip, index)+"="+chip.Status)
	}

	return strings.Join(lines, ", ")
}
