package steps

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoDocumentPath is returned when a clause names a document this suite holds
// no path for.
var ErrNoDocumentPath = errors.New("no path for that document")

// emptyDocumentBody is a VALID document holding none of the keys the outline
// reads, carrying a marker so a clause about the file's own content is about
// this document rather than anything that looks like it.
const emptyDocumentBody = "# %s\nnote: this document holds none of that document's keys\n"

// registerArchitectureShapeSteps binds the clauses about a document whose shape
// the outline does not know: the replacement, the marker it carries, and the
// emptiness every fixed group must then state.
func registerArchitectureShapeSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the ([a-z][a-z0-9-]*) document is replaced with a valid document `+
		`holding none of its keys$`, replaceDocumentWithEmptyOne)
	// One measurement, two phrasings: the marker clause is the seeded-token
	// clause, so the definition is reused rather than copied.
	suite.Step(`^(`+selectorPattern+`) contains the replaced document's marker$`,
		assertContainsSeededToken)
	suite.Step(`^every fixed outline group shows a non-empty ([a-z][a-z0-9-]*)$`,
		assertEveryGroupStatesEmptiness)
}

// replaceDocumentWithEmptyOne writes that document into the project tree the
// session scans, and files its marker where every marker clause reads one.
func replaceDocumentWithEmptyOne(state *State, args []string) error {
	relPath, err := documentPath(state, args[0])
	if err != nil {
		return err
	}

	if state.Tree == nil {
		return state.fail("%w", ErrNoProjectTree)
	}

	marker := fmt.Sprintf("tbdd-replaced-%s-%d", state.Scenario.ID, time.Now().UnixNano())

	err = disk.Write(filepath.Join(state.Tree.Dir, relPath),
		[]byte(fmt.Sprintf(emptyDocumentBody, marker)), disk.Shared)
	if err != nil {
		return state.fail("replacing %s: %w", relPath, err)
	}

	state.SeededToken, state.SeededPath = marker, relPath

	return nil
}

// documentPath is where the document a clause names lives in the project tree.
func documentPath(state *State, name string) (string, error) {
	if name == architectureNode {
		return canonicalArchitectureRel, nil
	}

	return "", state.fail("%w: %q", ErrNoDocumentPath, name)
}

// assertEveryGroupStatesEmptiness holds every outline group to saying it is
// empty in words: a bare header states nothing a reader can act on.
func assertEveryGroupStatesEmptiness(state *State, args []string) error {
	name := args[0]

	probe := fmt.Sprintf(`els => els.map(el => {
		const group = el.getAttribute(%[1]q) || "a group"
		const empty = el.querySelector('%[2]s')
		if (!empty) { return group + " shows no %[3]s" }
		const text = (empty.textContent || "").trim()
		return text === "" ? group + "'s %[3]s is empty" : %[4]q
	})`, groupAttribute, elementCSS(name, "", ""), name, verdictOK)

	return assertEveryElement(state, elementCSS(sidebarGroupTestID, "", ""), probe,
		"every fixed outline group must show a non-empty "+name)
}
