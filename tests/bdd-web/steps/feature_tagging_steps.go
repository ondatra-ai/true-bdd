package steps

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoSeededFeature is returned when a clause is about the feature a Given
// seeded and no Given seeded one.
var ErrNoSeededFeature = errors.New("no step seeded a feature into the product")

// ErrNoScenarioEntry is returned when the registry holds no entry with the id a
// clause names.
var ErrNoScenarioEntry = errors.New("the registry holds no such scenario entry")

// ErrNoFeatureField is returned when the document a clause names declares no
// feature at all, so there is nothing to compare.
var ErrNoFeatureField = errors.New("the document declares no feature")

const (
	// featuresNode is the node the features file declares its entries under, and
	// featureField the key a story or a registry entry carries its feature in.
	featuresNode = "features"
	featureField = "feature"
)

// registerFeatureTaggingSteps binds the vocabulary of tagging: the feature a
// Given seeds, the two ways a scenario picks one, and what the story or the
// registry document then carries.
func registerFeatureTaggingSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^a unique feature is seeded into the product's features$`, seedUniqueFeature)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) picks the seeded feature `+
			`in (`+selectorPattern+`)$`,
		pickSeededFeature)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) picks the feature "([^"]+)" `+
			`in (`+selectorPattern+`)$`,
		pickNamedFeature)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) replaces the story's feature `+
			`with the seeded one in the editor$`,
		replaceStoryFeature)
	suite.Step(`^the story "([^"]+)" has feature "([^"]+)"$`, assertStoryHasNamedFeature)
	suite.Step(`^the story "([^"]+)" has the seeded feature$`, assertStoryHasSeededFeature)
	suite.Step(`^the scenario "([^"]+)" in "([^"]+)" has the seeded feature$`,
		assertScenarioHasSeededFeature)
}

// seedUniqueFeature declares one more feature ON DISK by copying the first one
// and renaming the copy, so the entry is shaped like the document it lands in
// rather than like a guess at its schema.
func seedUniqueFeature(state *State, _ []string) error {
	raw, err := fixtureFile(state, featuresDocument)
	if err != nil {
		return err
	}

	feature := uniqueName(state, "feature")

	edited, err := withCopiedEntry(raw, featuresNode, feature)
	if err != nil {
		return state.fail("seeding a feature under %q of %s: %w",
			featuresNode, featuresDocument, err)
	}

	err = disk.Write(filepath.Join(state.Tree.Dir, featuresDocument),
		[]byte(edited), disk.Shared)
	if err != nil {
		return state.fail("writing %s: %w", featuresDocument, err)
	}

	state.SeededFeature = feature

	return nil
}

// pickSeededFeature tags through the picker the named container carries — a
// row's own, or the story page's. args[0] is the role, discarded as
// clickElement's is.
func pickSeededFeature(state *State, args []string) error {
	if state.SeededFeature == "" {
		return state.fail("%w", ErrNoSeededFeature)
	}

	return pickFeature(state, args[1], state.SeededFeature)
}

// pickNamedFeature is the same act against a feature the product already
// declares, which the step names.
func pickNamedFeature(state *State, args []string) error {
	return pickFeature(state, args[2], args[1])
}

// replaceStoryFeature retags the open story by hand: the seeded id replaces the
// value on the buffer's own feature line, which is the edit the picker makes
// through the UI.
func replaceStoryFeature(state *State, _ []string) error {
	if state.SeededFeature == "" {
		return state.fail("%w", ErrNoSeededFeature)
	}

	err := rememberAllowedDocuments(state)
	if err != nil {
		return err
	}

	raw, err := currentBuffer(state)
	if err != nil {
		return err
	}

	edited, err := withFeature(raw, state.SeededFeature)
	if err != nil {
		return state.fail("retagging the open story in the editor: %w", err)
	}

	return writeEditorBuffer(state, edited)
}

// withFeature is the buffer with its feature line carrying another id.
func withFeature(raw, feature string) (string, error) {
	// Compiled per call: a package-level regexp is a global.
	line := regexp.MustCompile(`(?m)^(\s*` + featureField + `:\s*).*$`)

	if !line.MatchString(raw) {
		return "", ErrNoFeatureField
	}

	return line.ReplaceAllString(raw, "${1}"+feature), nil
}

// assertStoryHasNamedFeature holds one story document to carrying the feature
// the step names.
func assertStoryHasNamedFeature(state *State, args []string) error {
	return assertStoryFeature(state, args[0], args[1])
}

// assertStoryHasSeededFeature is that clause against the feature the Given
// seeded, which the scenario cannot name because no run knows it in advance.
func assertStoryHasSeededFeature(state *State, args []string) error {
	if state.SeededFeature == "" {
		return state.fail("%w", ErrNoSeededFeature)
	}

	return assertStoryFeature(state, args[0], state.SeededFeature)
}

// assertStoryFeature holds a story document's own feature to a value. Polled:
// the save is the CLI's work and lands after the browser reports it.
func assertStoryFeature(state *State, relPath, want string) error {
	got, matched, err := await(readScalarField(state, relPath, featureField), equals(want))
	if err != nil {
		return state.fail("reading the feature of %s: %w", relPath, err)
	}

	if !matched {
		return state.fail("the feature of %s is %q, want %q", relPath, got, want)
	}

	return nil
}

// readScalarField reads one scalar of a document as a reader, so a clause polls
// the file rather than one reading of it.
func readScalarField(state *State, relPath, field string) func() (string, error) {
	return func() (string, error) { return fixtureField(state, relPath, field) }
}

// assertScenarioHasSeededFeature holds ONE registry entry to the seeded feature:
// the registry states many, so the entry is read out before the field is.
func assertScenarioHasSeededFeature(state *State, args []string) error {
	if state.SeededFeature == "" {
		return state.fail("%w", ErrNoSeededFeature)
	}

	scenarioID, relPath := args[0], args[1]

	got, matched, err := await(readEntryField(state, relPath, scenarioID, featureField),
		equals(state.SeededFeature))
	if err != nil {
		return state.fail("reading the feature of scenario %s in %s: %w",
			scenarioID, relPath, err)
	}

	if !matched {
		return state.fail("the feature of scenario %s in %s is %q, want %q",
			scenarioID, relPath, got, state.SeededFeature)
	}

	return nil
}

// readEntryField reads one scalar of one registry entry as a reader.
func readEntryField(state *State, relPath, scenarioID, field string) func() (string, error) {
	return func() (string, error) {
		raw, err := fixtureFile(state, relPath)
		if err != nil {
			return "", err
		}

		block, err := entryBlock(raw, scenarioID)
		if err != nil {
			return "", err
		}

		return scalarField(block, field)
	}
}

// entryBlock is the lines one registry entry owns: its `<id>:` line and
// everything indented under it.
func entryBlock(raw, scenarioID string) (string, error) {
	lines := strings.Split(raw, "\n")
	key := regexp.MustCompile(`^\s+` + regexp.QuoteMeta(scenarioID) + `:\s*$`)

	for index, line := range lines {
		if key.MatchString(line) {
			return strings.Join(blockOf(lines, index), "\n"), nil
		}
	}

	return "", fmt.Errorf("%w: %s", ErrNoScenarioEntry, scenarioID)
}
