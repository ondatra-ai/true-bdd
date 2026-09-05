package steps

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoStoryDocument is returned when the project tree holds no story file, so
// "each product document" would quietly be three documents rather than four.
var ErrNoStoryDocument = errors.New("the project tree holds no story document")

// ErrNoOpenedProductDocuments is returned when a clause grades the pages a When
// opened and no When opened any.
var ErrNoOpenedProductDocuments = errors.New("no step opened the product documents")

const (
	// prdDocument, featuresDocument and registryDocument are three of the four
	// product documents; the fourth is whichever story the tree holds.
	prdDocument      = "docs/product/product.yaml"
	featuresDocument = "docs/product/features.yaml"
	registryDocument = "docs/scenarios.yaml"
	// storiesGlob finds that story. Its id is the filename up to the first
	// hyphen ("60.1-summary-length-preference.yaml" is story 60.1).
	storiesGlob = "docs/product/stories/*.yaml"
)

// productDocument is one document of the product and the workspace target that
// opens it, resolved through workspaceRoute so this file states no route itself.
type productDocument struct {
	Target  string
	RelPath string
}

// registerProductDocumentSteps binds the vocabulary of a clause about ALL the
// product documents at once: the token seeded into each, the pass that opens
// each, and what each document's own page is then held to.
func registerProductDocumentSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^a unique token is seeded into each product document$`,
		seedTokenIntoEachProductDocument)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) opens each product `+
			`document in turn$`,
		openEachProductDocument)
	suite.Step(`^every product document's page names that document in (`+selectorPattern+`)$`,
		assertEachPageNamesDocument)
	suite.Step(`^every product document's page shows that document's seeded token$`,
		assertEachPageShowsSeededToken)
}

// productDocuments are the documents the product is made of, in the order a
// scenario opens them. Each names a workspace target, so the routes stay in
// workspaceRoute and this list cannot drift from them.
func productDocuments(state *State) ([]productDocument, error) {
	storyRel, err := firstStoryDocument(state)
	if err != nil {
		return nil, err
	}

	storyID, _, _ := strings.Cut(strings.TrimSuffix(filepath.Base(storyRel), ".yaml"), "-")

	return []productDocument{
		{Target: productNode, RelPath: prdDocument},
		{Target: featuresNode, RelPath: featuresDocument},
		{Target: "story " + storyID, RelPath: storyRel},
		{Target: scenariosNode, RelPath: registryDocument},
	}, nil
}

// firstStoryDocument is the story the tree holds, read from the tree rather than
// named by a scenario, so a fixture that renames its stories still resolves.
func firstStoryDocument(state *State) (string, error) {
	if state.Tree == nil {
		return "", state.fail("%w", ErrNoProjectTree)
	}

	pattern := filepath.Join(state.Tree.Dir, filepath.FromSlash(storiesGlob))

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", state.fail("globbing %s: %w", pattern, err)
	}

	if len(matches) == 0 {
		return "", state.fail("%w: %s matched nothing", ErrNoStoryDocument, pattern)
	}

	rel, err := filepath.Rel(state.Tree.Dir, matches[0])
	if err != nil {
		return "", state.fail("placing %s in the project tree: %w", matches[0], err)
	}

	return filepath.ToSlash(rel), nil
}

// seedTokenInto writes one marker into one document of the project tree, as its
// own comment line: legal wherever the document ends, and never on its last line.
func seedTokenInto(state *State, relPath, token string) error {
	raw, err := fixtureFile(state, relPath)
	if err != nil {
		return err
	}

	comment := fmt.Sprintf("# seeded by %s: %s\n", state.Scenario.ID, token)
	if raw != "" && !strings.HasSuffix(raw, "\n") {
		comment = "\n" + comment
	}

	err = disk.Append(filepath.Join(state.Tree.Dir, relPath), []byte(comment), disk.Shared)
	if err != nil {
		return state.fail("seeding a token into %s: %w", relPath, err)
	}

	return nil
}

// seedTokenIntoEachProductDocument seeds a DISTINCT marker into every product
// document, so a page rendering the wrong file's content cannot pass.
func seedTokenIntoEachProductDocument(state *State, _ []string) error {
	documents, err := productDocuments(state)
	if err != nil {
		return err
	}

	state.SeededTokens = map[string]string{}

	for index, document := range documents {
		token := fmt.Sprintf("tbdd-seed-%s-%d-%d",
			state.Scenario.ID, index, time.Now().UnixNano())

		err = seedTokenInto(state, document.RelPath, token)
		if err != nil {
			return err
		}

		state.SeededTokens[document.RelPath] = token
	}

	return nil
}

// openProductDocument navigates to one document's own page through the target
// map the workspace vocabulary already owns. openWorkspace discards the role it
// captures, so any value serves as args[0].
func openProductDocument(state *State, document productDocument) error {
	return openWorkspace(state, []string{"", document.Target})
}

// openEachProductDocument opens every product document in turn and records which,
// so the clauses after it grade the documents this pass visited.
func openEachProductDocument(state *State, _ []string) error {
	documents, err := productDocuments(state)
	if err != nil {
		return err
	}

	for _, document := range documents {
		err = openProductDocument(state, document)
		if err != nil {
			return err
		}
	}

	state.ProductDocuments = documents

	return nil
}

// openedProductDocuments are the documents the When visited.
func openedProductDocuments(state *State) ([]productDocument, error) {
	if len(state.ProductDocuments) == 0 {
		return nil, state.fail("%w", ErrNoOpenedProductDocuments)
	}

	return state.ProductDocuments, nil
}

// assertEachPageNamesDocument holds each document's own page to naming it. One
// page renders one document, so the clause is about four pages and re-opens each.
func assertEachPageNamesDocument(state *State, args []string) error {
	documents, err := openedProductDocuments(state)
	if err != nil {
		return err
	}

	for _, document := range documents {
		err = pageNamesDocument(state, args[0], document)
		if err != nil {
			return err
		}
	}

	return nil
}

// pageNamesDocument holds one document's page to carrying that document's path
// in the element the step names.
func pageNamesDocument(state *State, selectorText string, document productDocument) error {
	err := openProductDocument(state, document)
	if err != nil {
		return err
	}

	sel, locator, err := locateStep(state, selectorText)
	if err != nil {
		return err
	}

	got, matched, err := await(readInnerText(locator), containsAll([]string{document.RelPath}))
	if err != nil {
		return state.fail("%s on the page of %s: %w", sel, document.RelPath, err)
	}

	if !matched {
		return state.fail("%s on the page of %s reads %q, want it to name %s",
			sel, document.RelPath, got, document.RelPath)
	}

	return nil
}

// assertEachPageShowsSeededToken holds each page to rendering the marker seeded
// into THAT document — the file's own content, under a name no other file carries.
func assertEachPageShowsSeededToken(state *State, _ []string) error {
	documents, err := openedProductDocuments(state)
	if err != nil {
		return err
	}

	for _, document := range documents {
		err = pageShowsSeededToken(state, document)
		if err != nil {
			return err
		}
	}

	return nil
}

// pageShowsSeededToken re-opens one document and waits for its page to carry that
// document's marker anywhere it renders.
func pageShowsSeededToken(state *State, document productDocument) error {
	token := state.SeededTokens[document.RelPath]
	if token == "" {
		return state.fail("%w: %s", ErrNoSeededToken, document.RelPath)
	}

	err := openProductDocument(state, document)
	if err != nil {
		return err
	}

	page, err := state.page()
	if err != nil {
		return err
	}

	got, matched, err := await(readInnerText(page.Locator(bodyKey)), containsAll([]string{token}))
	if err != nil {
		return state.fail("reading the page of %s: %w", document.RelPath, err)
	}

	if !matched {
		return state.fail("the page of %s does not show its seeded token %q; it reads:\n%s",
			document.RelPath, token, got)
	}

	return nil
}
