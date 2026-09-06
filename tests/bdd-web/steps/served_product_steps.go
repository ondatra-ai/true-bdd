package steps

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoStoryFileForID is returned when the stories directory resolves the id a
// clause names to anything other than exactly one document.
var ErrNoStoryFileForID = errors.New("no single story document carries that id")

// registerServedProductSteps binds the clauses about what the workspace serves:
// the shape of the features file, the feature a story or a requirement carries,
// and the description a feature page renders.
func registerServedProductSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^every feature the served file holds has exactly the keys (`+keyListPattern+`)$`,
		assertServedFeatureKeys)
	suite.Step(`^the served story "([^"]+)" has a feature$`, assertServedStoryHasFeature)
	suite.Step(`^the served scenario "([^"]+)" has a feature$`, assertServedScenarioHasFeature)
	suite.Step(`^(`+selectorPattern+`) holds the description of "([^"]+)"$`,
		assertHoldsFeatureDescription)
}

// assertServedFeatureKeys holds every declared feature to exactly the keys the
// step names: a feature that also listed its stories would make the features
// file a second home for what the stories themselves state.
func assertServedFeatureKeys(state *State, args []string) error {
	features, err := productFeatures(state, featuresDocument)
	if err != nil {
		return err
	}

	if len(features) == 0 {
		return state.fail("%w: %s declares none", ErrNoFeatureEntry, featuresDocument)
	}

	want := strings.Join(keyNames(args[0]), ", ")

	for _, feature := range features {
		got := entryKeys(feature)
		if got != want {
			return state.fail("feature %q of %s carries the keys %s, want exactly %s",
				fmt.Sprint(feature["id"]), featuresDocument, got, want)
		}
	}

	return nil
}

// assertServedStoryHasFeature holds one story's own document to naming a feature
// at all — the reference that makes it findable under one.
func assertServedStoryHasFeature(state *State, args []string) error {
	relPath, err := storyDocument(state, args[0])
	if err != nil {
		return err
	}

	got, err := fixtureField(state, relPath, featureField)
	if err != nil {
		return state.fail("%w: %s", ErrNoFeatureField, relPath)
	}

	if strings.TrimSpace(got) == "" {
		return state.fail("%s carries an empty feature, want one naming a feature", relPath)
	}

	return nil
}

// storyDocument is the document of one story, found by the id its filename opens
// with, so a clause names the story rather than its path.
func storyDocument(state *State, storyID string) (string, error) {
	if state.Tree == nil {
		return "", state.fail("%w", ErrNoProjectTree)
	}

	glob := strings.Replace(storiesGlob, "*", storyID+"-*", 1)
	pattern := filepath.Join(state.Tree.Dir, filepath.FromSlash(glob))

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", state.fail("globbing %s: %w", pattern, err)
	}

	if len(matches) != 1 {
		return "", state.fail("%w: %s matched %d documents",
			ErrNoStoryFileForID, pattern, len(matches))
	}

	rel, err := filepath.Rel(state.Tree.Dir, matches[0])
	if err != nil {
		return "", state.fail("placing %s in the project tree: %w", matches[0], err)
	}

	return filepath.ToSlash(rel), nil
}

// assertServedScenarioHasFeature holds one registry entry to naming a feature,
// which is what puts a requirement under one rather than in the unaligned bucket.
func assertServedScenarioHasFeature(state *State, args []string) error {
	scenarioID := args[0]

	raw, err := fixtureFile(state, registryDocument)
	if err != nil {
		return err
	}

	block, err := entryBlock(raw, scenarioID)
	if err != nil {
		return state.fail("%s: %w", registryDocument, err)
	}

	got, err := scalarField(block, featureField)
	if err != nil {
		return state.fail("%w: scenario %s of %s",
			ErrNoFeatureField, scenarioID, registryDocument)
	}

	if strings.TrimSpace(got) == "" {
		return state.fail("scenario %s of %s carries an empty feature, "+
			"want one naming a feature", scenarioID, registryDocument)
	}

	return nil
}

// assertHoldsFeatureDescription holds an element to carrying the description the
// features file declares for one feature, read from the document so a fixture
// edit cannot drift from the clause.
func assertHoldsFeatureDescription(state *State, args []string) error {
	featureID := args[1]

	feature, err := featureByID(state, featuresDocument, featureID)
	if err != nil {
		return err
	}

	description, stated := feature["description"]
	if !stated {
		return state.fail("%w: %s declares no description for %s",
			ErrNoFixtureField, featuresDocument, featureID)
	}

	return assertHolds(state, args[0],
		fmt.Sprintf("the description of %q", featureID),
		[]string{fmt.Sprint(description)})
}
