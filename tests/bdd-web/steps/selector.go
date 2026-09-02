package steps

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/playwright-community/playwright-go"
)

// ErrMalformedSelector is returned when a step's element reference does
// not parse as `name[key=value] > child[key=value]`.
var ErrMalformedSelector = errors.New("not a step selector")

// selectorPattern is the element grammar every clause below shares, kept
// as a string so one grammar change moves every step pattern at once. Its
// groups are non-capturing: a step's own captures start after it.
const selectorPattern = `[a-z][a-z0-9-]*(?:\[[a-z][a-z0-9-]*=[^\]]+\])?(?: > [a-z][a-z0-9-]*)?`

// keyedChildSelectorPattern is the same grammar with a KEYED child, which
// selectorPattern's child term cannot take. The bracket is REQUIRED —
// optional would also match every bare-child step, which bddgo refuses.
const keyedChildSelectorPattern = `[a-z][a-z0-9-]*(?:\[[a-z][a-z0-9-]*=[^\]]+\])?` +
	` > [a-z][a-z0-9-]*\[[a-z][a-z0-9-]*=[^\]]+\]`

// selectorTimeout caps waiting for a step's element to render. The page
// polls its API and re-renders, so this is the budget for a client render
// rather than for the network.
const selectorTimeout = 15_000 // milliseconds

// selector is one step's element reference:
// `name[key=value] > child[key=value]`.
type selector struct {
	Name  string
	Key   string
	Value string

	Child      string
	ChildKey   string
	ChildValue string
}

// parseSelector splits a step's element reference into its parts.
func parseSelector(text string) (selector, error) {
	// Compiled per call: a package-level regexp is a global, and a step runs
	// a handful of times.
	grammar := regexp.MustCompile(
		`^([a-z][a-z0-9-]*)(?:\[([a-z][a-z0-9-]*)=([^\]]+)\])?` +
			`(?: > ([a-z][a-z0-9-]*)(?:\[([a-z][a-z0-9-]*)=([^\]]+)\])?)?$`)

	parts := grammar.FindStringSubmatch(strings.TrimSpace(text))
	if parts == nil {
		return selector{}, fmt.Errorf("%w: %q", ErrMalformedSelector, text)
	}

	return selector{
		Name: parts[1], Key: parts[2], Value: parts[3],
		Child: parts[4], ChildKey: parts[5], ChildValue: parts[6],
	}, nil
}

// String renders the selector as the step wrote it, so a failure names
// what the reader asked for rather than the CSS it became.
func (sel selector) String() string {
	text := sel.Name + attributeText(sel.Key, sel.Value)

	if sel.Child != "" {
		text += " > " + sel.Child + attributeText(sel.ChildKey, sel.ChildValue)
	}

	return text
}

// attributeText renders an element reference's `[key=value]`, or nothing
// when the reference carried none.
func attributeText(key, value string) string {
	if key == "" {
		return ""
	}

	return "[" + key + "=" + value + "]"
}

// locate resolves the selector against the scenario's page and waits for
// the element to be visible, so a clause reads a rendered element instead
// of racing the poll that renders it. Strict: two matches is a failure.
func (sel selector) locate(state *State) (playwright.Locator, error) {
	page, err := state.page()
	if err != nil {
		return nil, err
	}

	locator := sel.element(page)

	err = locator.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(selectorTimeout),
	})
	if err != nil {
		return nil, state.fail("the page never showed %s: %w\n%s", sel, err, visibleText(page))
	}

	return locator, nil
}

// element resolves the selector to a locator without waiting, so the
// presence clause and the absence clause share one testid contract instead
// of each spelling it out.
func (sel selector) element(page playwright.Page) playwright.Locator {
	locator := page.Locator(elementCSS(sel.Name, sel.Key, sel.Value))

	if sel.Child != "" {
		locator = locator.Locator(elementCSS(sel.Child, sel.ChildKey, sel.ChildValue))
	}

	return locator
}

// elementCSS renders one element reference as CSS. The child goes through
// it too, which is what lets two sections carry the same story testid: a
// child is looked up UNDER its root, never page-wide.
func elementCSS(name, key, value string) string {
	if key == "" {
		return fmt.Sprintf("[data-testid=%q]", name)
	}

	if dynamic, ok := dynamicTestID(name, key, value); ok {
		return fmt.Sprintf("[data-testid=%q]", dynamic)
	}

	return fmt.Sprintf("[data-testid=%q][data-%s=%q]", name, key, value)
}

// dynamicTestID answers whether the UI encodes this reference's key in the
// testid itself rather than in a data attribute, per the contract in
// tests/legacy/bdd-web-playwright/helpers/README-testids.md.
func dynamicTestID(name, key, value string) (string, bool) {
	// Keyed on the PAIR: story-row is dynamic under create-id and a plain
	// data attribute under the workspace's story-id.
	switch name + "/" + key {
	case "inventory-doc/key", "story-row/create-id":
		return name + "-" + value, true
	default:
		return "", false
	}
}

// domAttribute maps the name a step writes onto the attribute the UI
// renders: an aria- or data- name travels verbatim, a bare name is the
// data-* attribute of that name.
func domAttribute(name string) string {
	if strings.HasPrefix(name, "data-") || strings.HasPrefix(name, "aria-") {
		return name
	}

	return "data-" + name
}
