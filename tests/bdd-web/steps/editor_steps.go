package steps

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoServiceEntry is returned when the buffer declares no service to copy, or
// none by the name a step gives.
var ErrNoServiceEntry = errors.New("the buffer declares no such service entry")

// ErrNoEndpointItem is returned when the service a step names lists no endpoint
// to copy.
var ErrNoEndpointItem = errors.New("the service lists no endpoint to copy")

// ErrNoEditorBuffer is returned when a clause is about the buffer as the editor
// first held it and no earlier step read one.
var ErrNoEditorBuffer = errors.New("no step has read the editor's original buffer")

// ErrNoNewEndpoint is returned when a clause is about the endpoint a When
// declared and no When declared one.
var ErrNoNewEndpoint = errors.New("no step declared a new endpoint")

// ErrNoBoundingBox is returned when an element the geometry clause measures
// renders no box.
var ErrNoBoundingBox = errors.New("the element renders no box")

const (
	// editorSelector is the buffer every editor clause is about.
	editorSelector = "file-view-editor"
	// servicesNode is the top-level node a new service is declared under.
	servicesNode = "services"
	// focusPaintProperties are the computed properties the focus clause names,
	// longhand so a shorthand's parts are each compared.
	focusPaintProperties = "background-color border-top-width border-top-style " +
		"border-top-color outline-width outline-style outline-color box-shadow"
)

// registerEditorSteps binds the editor's vocabulary: the declarations a scenario
// types into the buffer, what the document then says, and the clauses about what
// focusing the buffer may not change.
func registerEditorSteps(suite *bddgo.Suite[State]) {
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) declares a "([^"]+)" `+
			`service in the editor$`,
		declareService)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) has declared a "([^"]+)" `+
			`service in the editor$`,
		declareService)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) restores the original `+
			`buffer in the editor$`,
		restoreEditorBuffer)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) declares a uniquely-named `+
			`endpoint for "([^"]+)" in the editor$`,
		declareEndpoint)
	suite.Step(`^the page shows the new endpoint under (`+selectorPattern+`)$`,
		assertNewEndpointShown)
	suite.Step(`^"([^"]+)" lists the new endpoint under the "([^"]+)" service$`,
		assertFileListsNewEndpoint)
	suite.Step(`^(`+selectorPattern+`) has computed "([^"]*)" = "([^"]*)"$`, assertComputedStyle)
	suite.Step(
		`^(`+selectorPattern+`)'s background, border, outline and shadow are unchanged$`,
		assertFocusPaintUnchanged)
	suite.Step(`^(`+selectorPattern+`) did not move within (`+selectorPattern+`)$`,
		assertDidNotMoveWithin)
}

// editorBuffer is what the editor currently holds, read through a probe that
// serves a form control and a contenteditable alike. The first reading is kept
// as the original, which is what the restore clause puts back.
func editorBuffer(state *State) (string, error) {
	_, locator, err := locateStep(state, editorSelector)
	if err != nil {
		return "", err
	}

	value, err := locator.Evaluate(`el => el.value ?? el.innerText`, nil)
	if err != nil {
		return "", state.fail("read the editor's buffer: %w", err)
	}

	text, ok := value.(string)
	if !ok {
		return "", state.fail("%w: %v", ErrUnreadableProbe, value)
	}

	if state.EditorOriginal == "" {
		state.EditorOriginal = text
	}

	return text, nil
}

// writeEditorBuffer types a whole buffer into the editor. Fill, not an assigned
// value: a value set from JavaScript fires no input event, and the page saves
// off that event.
func writeEditorBuffer(state *State, text string) error {
	_, locator, err := locateStep(state, editorSelector)
	if err != nil {
		return err
	}

	err = locator.Fill(text)
	if err != nil {
		return state.fail("typing the buffer into %s: %w", editorSelector, err)
	}

	return nil
}

// declareService adds one service by COPYING the first declared one and
// renaming the copy, so the new entry is shaped like the document it lands in
// rather than like a guess at its schema. args[0] is the role, discarded.
func declareService(state *State, args []string) error {
	name := args[1]

	raw, err := editorBuffer(state)
	if err != nil {
		return err
	}

	edited, err := withCopiedService(raw, name)
	if err != nil {
		return state.fail("declaring service %q in the editor: %w", name, err)
	}

	return writeEditorBuffer(state, edited)
}

// restoreEditorBuffer types back what the editor held before this scenario's
// first edit — how a scenario about a derived view being dropped undoes what it
// declared. args[0] is the role, discarded.
func restoreEditorBuffer(state *State, _ []string) error {
	if state.EditorOriginal == "" {
		return state.fail("%w", ErrNoEditorBuffer)
	}

	return writeEditorBuffer(state, state.EditorOriginal)
}

// declareEndpoint adds one endpoint to a named service, carrying a path no
// other run can have used, so the view clause and the file clause look for the
// same string. args[0] is the role, discarded.
func declareEndpoint(state *State, args []string) error {
	service := args[1]
	path := fmt.Sprintf("/probe-%d", time.Now().UnixNano())

	raw, err := editorBuffer(state)
	if err != nil {
		return err
	}

	edited, err := withCopiedEndpoint(raw, service, path)
	if err != nil {
		return state.fail("declaring an endpoint for %q in the editor: %w", service, err)
	}

	err = writeEditorBuffer(state, edited)
	if err != nil {
		return err
	}

	state.NewEndpointPath = path

	return nil
}

// withCopiedService returns the buffer with one more service: the first entry
// under the services node, repeated under the new name.
func withCopiedService(raw, name string) (string, error) {
	lines := strings.Split(raw, "\n")

	first, err := firstServiceEntry(lines)
	if err != nil {
		return "", err
	}

	block := blockOf(lines, first)
	copied := make([]string, len(block))
	copy(copied, block)
	copied[0] = indentOf(block[0]) + name + ":"

	return spliceAfter(lines, first+len(block), copied), nil
}

// withCopiedEndpoint returns the buffer with one more endpoint under the named
// service: its first endpoint, repeated with the path the caller names.
func withCopiedEndpoint(raw, service, path string) (string, error) {
	lines := strings.Split(raw, "\n")

	start, err := serviceEntry(lines, service)
	if err != nil {
		return "", err
	}

	item, err := firstEndpointItem(lines, start)
	if err != nil {
		return "", err
	}

	block := blockOf(lines, item)

	copied, err := withPath(block, path)
	if err != nil {
		return "", err
	}

	return spliceAfter(lines, item+len(block), copied), nil
}

// spliceAfter puts a block of lines at one index and joins the buffer back up.
func spliceAfter(lines []string, at int, block []string) string {
	out := make([]string, 0, len(lines)+len(block))
	out = append(out, lines[:at]...)
	out = append(out, block...)
	out = append(out, lines[at:]...)

	return strings.Join(out, "\n")
}

// firstServiceEntry is the index of the first entry under the top-level
// services node.
func firstServiceEntry(lines []string) (int, error) {
	index, err := firstEntryUnder(lines, servicesNode)
	if err != nil {
		return 0, ErrNoServiceEntry
	}

	return index, nil
}

// serviceEntry is the index of the line declaring one named service.
func serviceEntry(lines []string, service string) (int, error) {
	key := regexp.MustCompile(`^\s+` + regexp.QuoteMeta(service) + `:\s*$`)

	for index, line := range lines {
		if key.MatchString(line) {
			return index, nil
		}
	}

	return 0, fmt.Errorf("%w: %s", ErrNoServiceEntry, service)
}

// firstEndpointItem is the index of the first list item under the endpoints
// node of the service declared at start.
func firstEndpointItem(lines []string, start int) (int, error) {
	node := regexp.MustCompile(`^\s+endpoints:\s*$`)

	for offset, line := range blockOf(lines, start) {
		if !node.MatchString(line) {
			continue
		}

		item := start + offset + 1
		if item < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[item]), "- ") {
			return item, nil
		}

		break
	}

	return 0, fmt.Errorf("%w: %s", ErrNoEndpointItem, strings.TrimSpace(lines[start]))
}

// withPath is one endpoint item with its path replaced.
func withPath(block []string, path string) ([]string, error) {
	copied := make([]string, len(block))
	copy(copied, block)

	key := regexp.MustCompile(`^(\s*-?\s*path:\s*).*$`)

	for index, line := range copied {
		if key.MatchString(line) {
			copied[index] = key.ReplaceAllString(line, "${1}"+path)

			return copied, nil
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrNoEndpointItem, strings.Join(block, " / "))
}

// blockOf is the lines one entry owns: its own line and everything indented
// deeper, up to the next line that is not.
func blockOf(lines []string, start int) []string {
	indent := indentOf(lines[start])
	end := start + 1

	for ; end < len(lines); end++ {
		if strings.TrimSpace(lines[end]) == "" {
			break
		}

		if len(indentOf(lines[end])) <= len(indent) {
			break
		}
	}

	return lines[start:end]
}

// indentOf is a line's leading whitespace.
func indentOf(line string) string {
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}

// assertNewEndpointShown holds the derived view to carrying the endpoint the
// When declared, found by the path it alone has.
func assertNewEndpointShown(state *State, args []string) error {
	if state.NewEndpointPath == "" {
		return state.fail("%w", ErrNoNewEndpoint)
	}

	return assertElementContainsText(state, []string{args[0], state.NewEndpointPath})
}

// assertFileListsNewEndpoint holds the document on disk to listing that same
// endpoint under that same service. Polled: the save is the CLI's work and
// lands after the browser reports it.
func assertFileListsNewEndpoint(state *State, args []string) error {
	if state.NewEndpointPath == "" {
		return state.fail("%w", ErrNoNewEndpoint)
	}

	relPath, service := args[0], args[1]
	want := state.NewEndpointPath

	got, matched, err := await(readServiceBlock(state, relPath, service),
		func(value string) bool { return strings.Contains(value, want) })
	if err != nil {
		return state.fail("reading the %q service of %s: %w", service, relPath, err)
	}

	if !matched {
		return state.fail("the %q service of %s does not list %q; it reads:\n%s",
			service, relPath, want, got)
	}

	return nil
}

// readServiceBlock reads one service's own lines out of the document in the
// project tree, so the clause polls the file rather than one reading of it.
func readServiceBlock(state *State, relPath, service string) func() (string, error) {
	return func() (string, error) {
		raw, err := fixtureFile(state, relPath)
		if err != nil {
			return "", err
		}

		lines := strings.Split(raw, "\n")

		start, err := serviceEntry(lines, service)
		if err != nil {
			return "", err
		}

		return strings.Join(blockOf(lines, start), "\n"), nil
	}
}

// assertComputedStyle holds one computed property to the value the step names.
func assertComputedStyle(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	property, want := args[1], args[2]

	got, matched, err := await(readComputedStyle(locator, property), equals(want))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s has computed %s = %q, want %q", sel, property, got, want)
	}

	return nil
}

// readComputedStyle reads one computed property as the browser resolves it.
func readComputedStyle(locator playwright.Locator, property string) func() (string, error) {
	script := fmt.Sprintf(`el => getComputedStyle(el).getPropertyValue(%q).trim()`, property)

	return func() (string, error) {
		value, err := locator.Evaluate(script, nil)
		if err != nil {
			return "", fmt.Errorf("read computed %s: %w", property, err)
		}

		text, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("%w: %v", ErrUnreadableProbe, value)
		}

		return text, nil
	}
}

// assertFocusPaintUnchanged holds focusing the element to painting it exactly as
// it was. The clause is a comparison, so it takes both readings itself and
// leaves focus where the When left it.
func assertFocusPaintUnchanged(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	focused, blurred, err := aroundFocus(locator, readPaint(locator))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if focused != blurred {
		return state.fail("%s paints differently while it holds focus:\n focused: %s\n blurred: %s",
			sel, focused, blurred)
	}

	return nil
}

// assertDidNotMoveWithin holds an element's box inside its container to being
// the same focused and blurred: editing happens in place, so focus may reflow
// nothing.
func assertDidNotMoveWithin(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	container, containerLocator, err := locateStep(state, args[1])
	if err != nil {
		return err
	}

	focused, blurred, err := aroundFocus(locator, readRelativeBox(locator, containerLocator))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if focused != blurred {
		return state.fail("%s sits at %s within %s while focused and at %s while blurred",
			sel, focused, container, blurred)
	}

	return nil
}

// aroundFocus reads one value with the element focused and again with it
// blurred, then leaves it focused: every clause here is about what focus
// changes, which one reading cannot answer.
func aroundFocus(locator playwright.Locator,
	read func() (string, error),
) (string, string, error) {
	err := locator.Focus()
	if err != nil {
		return "", "", fmt.Errorf("focus the element: %w", err)
	}

	focused, err := read()
	if err != nil {
		return "", "", err
	}

	_, err = locator.Evaluate(`el => el.blur()`, nil)
	if err != nil {
		return "", "", fmt.Errorf("blur the element: %w", err)
	}

	blurred, err := read()
	if err != nil {
		return "", "", err
	}

	_, err = locator.Evaluate(`el => el.focus()`, nil)
	if err != nil {
		return "", "", fmt.Errorf("restore focus: %w", err)
	}

	return focused, blurred, nil
}

// readPaint reads the properties the focus clause names as one string, so a
// difference names the property that changed.
func readPaint(locator playwright.Locator) func() (string, error) {
	script := fmt.Sprintf(`el => { const style = getComputedStyle(el); `+
		`return %q.split(" ").map(name => name + "=" + `+
		`style.getPropertyValue(name).trim()).join(" ") }`, focusPaintProperties)

	return func() (string, error) {
		value, err := locator.Evaluate(script, nil)
		if err != nil {
			return "", fmt.Errorf("read how the element is painted: %w", err)
		}

		text, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("%w: %v", ErrUnreadableProbe, value)
		}

		return text, nil
	}
}

// readRelativeBox is where one element sits inside another and how big it is,
// rounded to a tenth of a pixel so sub-pixel noise is not a move.
func readRelativeBox(inner, outer playwright.Locator) func() (string, error) {
	return func() (string, error) {
		innerBox, err := inner.BoundingBox()
		if err != nil {
			return "", fmt.Errorf("measure the element: %w", err)
		}

		outerBox, err := outer.BoundingBox()
		if err != nil {
			return "", fmt.Errorf("measure the container: %w", err)
		}

		if innerBox == nil || outerBox == nil {
			return "", ErrNoBoundingBox
		}

		return fmt.Sprintf("%.1f,%.1f %.1fx%.1f",
			innerBox.X-outerBox.X, innerBox.Y-outerBox.Y,
			innerBox.Width, innerBox.Height), nil
	}
}
