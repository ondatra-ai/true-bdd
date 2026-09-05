package steps

import (
	"errors"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoEditedDocument is returned when a clause is about the documents a When
// edited and no When edited any.
var ErrNoEditedDocument = errors.New("no step edited a document")

// documentSeparator is how a step joins the documents it names one after another.
const documentSeparator = " and to the "

// editedDocument is one document a When typed a comment into, with the comment it
// typed — what every clause after it is held to.
type editedDocument struct {
	Document productDocument
	Comment  string
}

// registerMultiDocumentEditSteps binds the vocabulary of an edit made to SEVERAL
// documents in turn: the comment typed into each, and what each document's
// editor, bytes and parse then say.
func registerMultiDocumentEditSteps(suite *bddgo.Suite[State]) {
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) appends a unique comment `+
			`to the (.+) in turn$`,
		appendCommentToDocumentsInTurn)
	suite.Step(`^every edited document's editor shows its comment$`,
		assertEditedEditorsShowComment)
	suite.Step(`^every edited document holds its comment on disk$`,
		assertEditedDocumentsHoldComment)
	suite.Step(`^every edited document still parses$`, assertEditedDocumentsParse)
}

// namedDocument maps the name a step writes onto the document it means and the
// workspace target that opens it.
func namedDocument(state *State, name string) (productDocument, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "prd":
		return productDocument{Target: productNode, RelPath: prdDocument}, nil
	case "features file", featuresNode:
		return productDocument{Target: featuresNode, RelPath: featuresDocument}, nil
	case "registry", "scenarios file", scenariosNode:
		return productDocument{Target: scenariosNode, RelPath: registryDocument}, nil
	default:
		return productDocument{}, state.fail("%w: %q", ErrNoWorkspaceTarget, name)
	}
}

// appendCommentToDocumentsInTurn edits each document the step names, one after
// another, and records what it typed where.
func appendCommentToDocumentsInTurn(state *State, args []string) error {
	state.EditedDocuments = nil

	for _, name := range strings.Split(args[1], documentSeparator) {
		document, err := namedDocument(state, name)
		if err != nil {
			return err
		}

		comment, err := appendCommentToDocument(state, document)
		if err != nil {
			return err
		}

		state.EditedDocuments = append(state.EditedDocuments,
			editedDocument{Document: document, Comment: comment})
	}

	return nil
}

// appendCommentToDocument types a comment no other run can have produced into one
// document, then waits for the write to be ANSWERED: a save still in flight is a
// save the next document's navigation loses, and that navigation resets the log.
func appendCommentToDocument(state *State, document productDocument) (string, error) {
	err := openProductDocument(state, document)
	if err != nil {
		return "", err
	}

	err = rememberAllowedDocuments(state)
	if err != nil {
		return "", err
	}

	raw, err := editorBuffer(state)
	if err != nil {
		return "", err
	}

	comment := "# " + uniqueName(state, "comment")

	err = writeEditorBuffer(state, endedWithNewline(raw)+comment+"\n")
	if err != nil {
		return "", err
	}

	_, err = awaitAnsweredWrite(state)

	return comment, err
}

// editedDocuments are the documents the When typed into.
func editedDocuments(state *State) ([]editedDocument, error) {
	if len(state.EditedDocuments) == 0 {
		return nil, state.fail("%w", ErrNoEditedDocument)
	}

	return state.EditedDocuments, nil
}

// assertEditedEditorsShowComment re-opens each edited document and holds its
// editor to still carrying the comment typed into it.
func assertEditedEditorsShowComment(state *State, _ []string) error {
	edits, err := editedDocuments(state)
	if err != nil {
		return err
	}

	for _, edit := range edits {
		err = editorShowsComment(state, edit)
		if err != nil {
			return err
		}
	}

	return nil
}

// editorShowsComment is that clause for one document.
func editorShowsComment(state *State, edit editedDocument) error {
	err := openProductDocument(state, edit.Document)
	if err != nil {
		return err
	}

	got, matched, err := await(editorReader(state), containsAll([]string{edit.Comment}))
	if err != nil {
		return state.fail("reading the editor of %s: %w", edit.Document.RelPath, err)
	}

	if !matched {
		return state.fail("the editor of %s does not show %q; it reads:\n%s",
			edit.Document.RelPath, edit.Comment, got)
	}

	return nil
}

// editorReader reads the editor's buffer as a reader, so a clause polls it through
// the same await every value clause uses.
func editorReader(state *State) func() (string, error) {
	return func() (string, error) { return editorBuffer(state) }
}

// assertEditedDocumentsHoldComment holds each document ON DISK to the comment
// typed into it. Polled: the save is the CLI's work and lands after the browser
// reports it.
func assertEditedDocumentsHoldComment(state *State, _ []string) error {
	edits, err := editedDocuments(state)
	if err != nil {
		return err
	}

	for _, edit := range edits {
		relPath := edit.Document.RelPath

		got, matched, readErr := await(readDocument(state, relPath),
			containsAll([]string{edit.Comment}))
		if readErr != nil {
			return state.fail("reading %s: %w", relPath, readErr)
		}

		if !matched {
			return state.fail("%s does not hold %q; it reads:\n%s", relPath, edit.Comment, got)
		}
	}

	return nil
}

// assertEditedDocumentsParse holds every edited document to still being YAML: an
// edit that persists but breaks the document is not a save this workspace may make.
func assertEditedDocumentsParse(state *State, _ []string) error {
	edits, err := editedDocuments(state)
	if err != nil {
		return err
	}

	for _, edit := range edits {
		relPath := edit.Document.RelPath

		raw, readErr := fixtureFile(state, relPath)
		if readErr != nil {
			return readErr
		}

		var parsed any

		parseErr := yaml.Unmarshal([]byte(raw), &parsed)
		if parseErr != nil {
			return state.fail("%s no longer parses as YAML after the edit: %w", relPath, parseErr)
		}
	}

	return nil
}
