package steps

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoNodeEntry is returned when the buffer declares no node by the name a
// step gives, or none with an entry to copy.
var ErrNoNodeEntry = errors.New("the buffer declares no such node")

// ErrNoNamedEntry is returned when the entry a step would copy carries no name
// to rename, so the copy could not be told from the original.
var ErrNoNamedEntry = errors.New("the entry carries no name to rename")

// ErrNoNewTerm is returned when a clause is about the term a When typed and no
// When typed one.
var ErrNoNewTerm = errors.New("no step added a term to the editor")

// ErrNoNewService is returned when a clause is about the service a When
// declared and no When declared one.
var ErrNoNewService = errors.New("no step declared a uniquely-named service")

// ErrNoDocumentSnapshot is returned when the unchanged clause runs and no When
// edited the buffer, which would leave it comparing nothing.
var ErrNoDocumentSnapshot = errors.New("no step edited the editor's buffer")

const (
	// termsNode is the node a term is declared under, and archDocument the only
	// document declaring one — the workspace these scenarios have open.
	termsNode    = "terms"
	archDocument = "docs/architecture/architecture.yaml"
	// brokenYAMLLine is the unterminated flow sequence a scenario breaks the
	// buffer with: the smallest edit that cannot parse.
	brokenYAMLLine = "broken: [unterminated\n"
	// killSignal ends the CLI outright — what "the remote is stopped" means when
	// the step names no signal.
	killSignal = "SIGKILL"
)

// registerDocumentEditSteps binds the persistence vocabulary's editor side: the
// edits a scenario types, the reload and the stop that follow them, and what the
// documents on disk are then held to.
func registerDocumentEditSteps(suite *bddgo.Suite[State]) {
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) adds a uniquely-named `+
			`term in the editor$`,
		addTerm)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) has added a uniquely-named `+
			`term in the editor$`,
		addTermAndAwaitSave)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) declares a uniquely-named `+
			`service in the editor$`,
		declareUniqueService)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) appends a unique comment `+
			`in the editor$`,
		appendUniqueComment)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) breaks the editor's YAML `+
			`with an unterminated sequence$`,
		breakEditorYAML)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) has broken the editor's `+
			`YAML with an unterminated sequence$`,
		breakEditorYAML)
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) reloads the page$`,
		reloadPage)
	suite.Step(`^the remote is stopped$`, stopRemote)
	suite.Step(`^"([^"]+)" holds the new term under its "([^"]+)" node$`, assertFileHoldsNewTerm)
	suite.Step(`^"([^"]+)" holds the new service$`, assertFileHoldsNewService)
	suite.Step(`^(`+selectorPattern+`) contains the new term$`, assertContainsNewTerm)
	suite.Step(`^no allowed document changed$`, assertNoAllowedDocumentChanged)
	suite.Step(`^"([^"]+)" holds the comment$`, assertDocumentHoldsComment)
	suite.Step(`^"([^"]+)" still parses$`, assertDocumentParses)
	suite.Step(`^(`+selectorPattern+`) has attribute "([^"]*)" matching (.+)$`,
		assertAttributeMatches)
}

// uniqueName is a marker no other run can have produced, shaped as a YAML key:
// lower case, digits and hyphens only.
func uniqueName(state *State, kind string) string {
	return fmt.Sprintf("tbdd-%s-%s-%d", kind,
		strings.ToLower(state.Scenario.ID), time.Now().UnixNano())
}

// currentBuffer is the editor's buffer with the unterminated line this suite may
// have typed removed: correcting a broken buffer IS the next edit, and the
// correction is the removal of exactly the line the break step typed.
func currentBuffer(state *State) (string, error) {
	raw, err := editorBuffer(state)
	if err != nil {
		return "", err
	}

	return strings.ReplaceAll(raw, brokenYAMLLine, ""), nil
}

// addTerm declares one more term by copying the first one and renaming the copy,
// so the new entry is shaped like the document it lands in. args[0] is the role,
// discarded as declareService's is.
func addTerm(state *State, _ []string) error {
	err := rememberAllowedDocuments(state)
	if err != nil {
		return err
	}

	raw, err := currentBuffer(state)
	if err != nil {
		return err
	}

	term := uniqueName(state, "term")

	edited, err := withCopiedEntry(raw, termsNode, term)
	if err != nil {
		return state.fail("adding a term under %q in the editor: %w", termsNode, err)
	}

	err = writeEditorBuffer(state, edited)
	if err != nil {
		return err
	}

	state.NewTerm = term

	return nil
}

// addTermAndAwaitSave is the Given form: it also waits for the document to hold
// the term, because a precondition a later step can still race is not one.
func addTermAndAwaitSave(state *State, args []string) error {
	err := addTerm(state, args)
	if err != nil {
		return err
	}

	return assertFileHoldsNewTerm(state, []string{archDocument, termsNode})
}

// declareUniqueService declares one service the scenario does not name, through
// the copy-and-rename declareService already does.
func declareUniqueService(state *State, _ []string) error {
	err := rememberAllowedDocuments(state)
	if err != nil {
		return err
	}

	raw, err := currentBuffer(state)
	if err != nil {
		return err
	}

	name := uniqueName(state, "service")

	edited, err := withCopiedService(raw, name)
	if err != nil {
		return state.fail("declaring service %q in the editor: %w", name, err)
	}

	err = writeEditorBuffer(state, edited)
	if err != nil {
		return err
	}

	state.NewServiceName = name

	return nil
}

// appendUniqueComment types a comment no other run can have produced: an edit
// that changes the document's bytes and nothing it means.
func appendUniqueComment(state *State, _ []string) error {
	err := rememberAllowedDocuments(state)
	if err != nil {
		return err
	}

	raw, err := currentBuffer(state)
	if err != nil {
		return err
	}

	comment := "# " + uniqueName(state, "comment")
	state.AppendedComment = comment

	return writeEditorBuffer(state, endedWithNewline(raw)+comment+"\n")
}

// assertDocumentHoldsComment holds one document ON DISK to the comment the When
// typed into its editor. Polled: the save is the CLI's work and lands after the
// browser reports it.
func assertDocumentHoldsComment(state *State, args []string) error {
	if state.AppendedComment == "" {
		return state.fail("%w", ErrNoEditedDocument)
	}

	relPath := args[0]

	got, matched, err := await(readDocument(state, relPath),
		containsAll([]string{state.AppendedComment}))
	if err != nil {
		return state.fail("reading %s: %w", relPath, err)
	}

	if !matched {
		return state.fail("%s does not hold %q; it reads:\n%s",
			relPath, state.AppendedComment, got)
	}

	return nil
}

// assertDocumentParses holds one document to still being YAML: an edit that
// persists but breaks the document is not a save this workspace may make.
func assertDocumentParses(state *State, args []string) error {
	relPath := args[0]

	raw, err := fixtureFile(state, relPath)
	if err != nil {
		return err
	}

	var parsed any

	err = yaml.Unmarshal([]byte(raw), &parsed)
	if err != nil {
		return state.fail("%s no longer parses as YAML after the edit: %w", relPath, err)
	}

	return nil
}

// breakEditorYAML types a flow sequence that is never closed — an edit no
// reformatting rescues, which is what an unparseable buffer means here.
func breakEditorYAML(state *State, _ []string) error {
	err := rememberAllowedDocuments(state)
	if err != nil {
		return err
	}

	raw, err := editorBuffer(state)
	if err != nil {
		return err
	}

	return writeEditorBuffer(state, endedWithNewline(raw)+brokenYAMLLine)
}

// endedWithNewline is the buffer with a final newline, so what is appended lands
// as its own line rather than on the last one.
func endedWithNewline(raw string) string {
	if raw == "" || strings.HasSuffix(raw, "\n") {
		return raw
	}

	return raw + "\n"
}

// reloadPage re-requests the open document, which is how a scenario asks whether
// the page reads the document or remembers it.
func reloadPage(state *State, _ []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	_, err = page.Reload(playwright.PageReloadOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	})
	if err != nil {
		return state.fail("%w: reloading %s: %w", ErrNavigation, page.URL(), err)
	}

	return nil
}

// stopRemote ends the CLI outright — the unnamed-signal form of the clause the
// signal vocabulary already serves.
func stopRemote(state *State, _ []string) error {
	return stopRemoteWithSignal(state, []string{killSignal})
}

// withCopiedEntry returns the buffer with one more entry under a top-level node:
// its first entry, repeated under a new name.
func withCopiedEntry(raw, node, name string) (string, error) {
	lines := strings.Split(raw, "\n")

	first, err := firstEntryUnder(lines, node)
	if err != nil {
		return "", err
	}

	block := blockOf(lines, first)

	copied, err := renamedEntry(block, name)
	if err != nil {
		return "", err
	}

	return spliceAfter(lines, first+len(block), copied), nil
}

// renamedEntry is one entry block under a new name: a sequence item is renamed
// through its first key's value, a mapping entry through the key itself.
func renamedEntry(block []string, name string) ([]string, error) {
	copied := make([]string, len(block))
	copy(copied, block)

	item := regexp.MustCompile(`^(\s*-\s+[a-z_-]+:\s*).*$`)
	key := regexp.MustCompile(`^(\s*)[^\s:]+:(.*)$`)

	switch {
	case item.MatchString(copied[0]):
		copied[0] = item.ReplaceAllString(copied[0], "${1}"+name)
	case key.MatchString(copied[0]):
		copied[0] = key.ReplaceAllString(copied[0], "${1}"+name+":${2}")
	default:
		return nil, fmt.Errorf("%w: %s", ErrNoNamedEntry, strings.TrimSpace(copied[0]))
	}

	return copied, nil
}

// firstEntryUnder is the index of the first entry under a top-level node.
func firstEntryUnder(lines []string, node string) (int, error) {
	// Compiled per call: a package-level regexp is a global.
	key := regexp.MustCompile(`^` + regexp.QuoteMeta(node) + `:\s*$`)

	for index, line := range lines {
		if !key.MatchString(line) {
			continue
		}

		for next := index + 1; next < len(lines); next++ {
			if strings.TrimSpace(lines[next]) == "" {
				continue
			}

			if indentOf(lines[next]) == "" {
				break
			}

			return next, nil
		}
	}

	return 0, fmt.Errorf("%w: %s", ErrNoNodeEntry, node)
}

// nodeBlock is the lines one top-level node owns: its own line and everything
// indented under it.
func nodeBlock(lines []string, node string) (string, error) {
	key := regexp.MustCompile(`^` + regexp.QuoteMeta(node) + `:\s*$`)

	for index, line := range lines {
		if !key.MatchString(line) {
			continue
		}

		end := index + 1
		for ; end < len(lines); end++ {
			if strings.TrimSpace(lines[end]) == "" {
				continue
			}

			if indentOf(lines[end]) == "" {
				break
			}
		}

		return strings.Join(lines[index:end], "\n"), nil
	}

	return "", fmt.Errorf("%w: %s", ErrNoNodeEntry, node)
}

// assertFileHoldsNewTerm holds the document on disk to declaring the term the
// When typed, under the node the step names. Polled: the save is the CLI's work
// and lands after the browser reports it.
func assertFileHoldsNewTerm(state *State, args []string) error {
	if state.NewTerm == "" {
		return state.fail("%w", ErrNoNewTerm)
	}

	relPath, node := args[0], args[1]

	got, matched, err := await(readNodeBlock(state, relPath, node),
		func(value string) bool { return strings.Contains(value, state.NewTerm) })
	if err != nil {
		return state.fail("reading the %q node of %s: %w", node, relPath, err)
	}

	if !matched {
		return state.fail("the %q node of %s does not hold %q; it reads:\n%s",
			node, relPath, state.NewTerm, got)
	}

	state.SavedPath = relPath

	return nil
}

// readNodeBlock reads one node's own lines out of the document in the project
// tree, so the clause polls the file rather than one reading of it.
func readNodeBlock(state *State, relPath, node string) func() (string, error) {
	return func() (string, error) {
		raw, err := fixtureFile(state, relPath)
		if err != nil {
			return "", err
		}

		return nodeBlock(strings.Split(raw, "\n"), node)
	}
}

// assertFileHoldsNewService holds the document on disk to declaring the service
// the When declared, polled for the same reason.
func assertFileHoldsNewService(state *State, args []string) error {
	if state.NewServiceName == "" {
		return state.fail("%w", ErrNoNewService)
	}

	relPath := args[0]

	got, matched, err := await(readDocument(state, relPath),
		func(value string) bool { return strings.Contains(value, state.NewServiceName) })
	if err != nil {
		return state.fail("reading %s: %w", relPath, err)
	}

	if !matched {
		return state.fail("%s does not hold %q; it reads:\n%s",
			relPath, state.NewServiceName, got)
	}

	state.SavedPath = relPath

	return nil
}

// readDocument reads one document of the project tree as a reader, so a content
// clause polls the file through the same await every value clause uses.
func readDocument(state *State, relPath string) func() (string, error) {
	return func() (string, error) { return fixtureFile(state, relPath) }
}

// assertContainsNewTerm holds an element's text to carrying the term the When
// typed, which is the containment clause under a different name.
func assertContainsNewTerm(state *State, args []string) error {
	if state.NewTerm == "" {
		return state.fail("%w", ErrNoNewTerm)
	}

	return assertElementContainsText(state, []string{args[0], state.NewTerm})
}

// allowedDocuments are the documents the CLI will write at all: the four fixed
// ones plus the story files (services/bdd-cli/internal/app/remote/docs.go).
func allowedDocuments(state *State) ([]string, error) {
	if state.Tree == nil {
		return nil, state.fail("%w", ErrNoProjectTree)
	}

	paths := []string{
		archDocument,
		"docs/product/product.yaml",
		"docs/product/features.yaml",
		"docs/scenarios.yaml",
	}

	matches, err := filepath.Glob(
		filepath.Join(state.Tree.Dir, "docs", productNode, "stories", "*.yaml"))
	if err != nil {
		return nil, state.fail("listing the story documents: %w", err)
	}

	for _, match := range matches {
		rel, relErr := filepath.Rel(state.Tree.Dir, match)
		if relErr != nil {
			return nil, state.fail("placing %s in the project tree: %w", match, relErr)
		}

		paths = append(paths, filepath.ToSlash(rel))
	}

	return paths, nil
}

// rememberAllowedDocuments snapshots those documents before a scenario's FIRST
// edit, which is the only reading the unchanged clause can be compared to.
func rememberAllowedDocuments(state *State) error {
	if state.DocsBefore != nil {
		return nil
	}

	before, err := readAllowedDocuments(state)
	if err != nil {
		return err
	}

	state.DocsBefore = before

	return nil
}

// readAllowedDocuments reads all of them at once; an unreadable one reads as
// empty, so a document CREATED after the snapshot still shows as a change.
func readAllowedDocuments(state *State) (map[string]string, error) {
	paths, err := allowedDocuments(state)
	if err != nil {
		return nil, err
	}

	documents := map[string]string{}

	for _, relPath := range paths {
		raw, readErr := fixtureFile(state, relPath)
		if readErr != nil {
			raw = ""
		}

		documents[relPath] = raw
	}

	return documents, nil
}

// assertNoAllowedDocumentChanged holds every writable document to the bytes it
// had before the edit. Watched rather than read once: a write that lands a beat
// late would pass a single reading.
func assertNoAllowedDocumentChanged(state *State, _ []string) error {
	if state.DocsBefore == nil {
		return state.fail("%w", ErrNoDocumentSnapshot)
	}

	deadline := time.Now().Add(attributeSettle)

	for {
		now, err := readAllowedDocuments(state)
		if err != nil {
			return err
		}

		changed, before, after := firstChange(state.DocsBefore, now)
		if changed != "" {
			return state.fail("%s changed: it held %d bytes and now holds %d, "+
				"want every allowed document untouched", changed, len(before), len(after))
		}

		if !time.Now().Before(deadline) {
			return nil
		}

		time.Sleep(valuePollInterval)
	}
}

// firstChange names the first document whose bytes moved, with both readings, so
// a failure says WHAT changed rather than only that something did.
func firstChange(before, after map[string]string) (string, string, string) {
	for relPath, was := range before {
		now, listed := after[relPath]
		if !listed || now != was {
			return relPath, was, now
		}
	}

	for relPath, now := range after {
		_, listed := before[relPath]
		if !listed {
			return relPath, "", now
		}
	}

	return "", "", ""
}

// assertAttributeMatches holds one attribute to the step's regexp — the clause a
// scenario writes when either of two readings is the outcome it means.
func assertAttributeMatches(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	name := domAttribute(args[1])

	pattern, err := regexp.Compile(args[2])
	if err != nil {
		return state.fail("the step's pattern %q does not compile: %w", args[2], err)
	}

	got, matched, err := await(readAttribute(locator, name), pattern.MatchString)
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s has %s = %q, want one matching %s", sel, name, got, pattern)
	}

	return nil
}
