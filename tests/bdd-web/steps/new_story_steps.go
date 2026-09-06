package steps

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoCreatedStory is returned when a clause is about the story the form wrote
// and the tree holds no story the snapshot did not.
var ErrNoCreatedStory = errors.New("no story document was created")

// ErrManyCreatedStories is returned when several stories appeared, so "the
// created story" names no single document.
var ErrManyCreatedStories = errors.New("more than one story document was created")

// ErrNoCoinedFeature is returned when a clause is about the feature a When
// coined and no When coined one.
var ErrNoCoinedFeature = errors.New("no step coined a uniquely-named feature")

const (
	// newStoryOpen is the control that opens the form, newStoryForm the form
	// itself, and the other two the controls it carries.
	newStoryOpen   = "new-story-open"
	newStoryForm   = "new-story-form"
	newStoryTitle  = "new-story-title"
	newStorySubmit = "new-story-submit"
)

// registerNewStorySteps binds the new-story form's vocabulary: opening it, the
// two ways a story is created through it, and what the documents on disk are
// then held to.
func registerNewStorySteps(suite *bddgo.Suite[State]) {
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) has opened the new-story form$`,
		openNewStoryForm)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) creates a story titled `+
			`"([^"]*)" under the feature "([^"]+)"$`,
		createStoryUnderFeature)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) creates a story titled `+
			`"([^"]*)" under a uniquely-named new feature$`,
		createStoryUnderCoinedFeature)
	suite.Step(`^the created story has feature "([^"]+)"$`, assertCreatedStoryFeature)
	suite.Step(`^the created story has the new feature$`, assertCreatedStoryCoinedFeature)
	suite.Step(`^"([^"]+)" holds the new feature with exactly the keys (`+keyListPattern+`)$`,
		assertFileHoldsCoinedFeature)
}

// openNewStoryForm opens the form unless it already stands open, snapshotting
// the writable documents first: the unchanged clause has no other baseline.
func openNewStoryForm(state *State, _ []string) error {
	err := rememberAllowedDocuments(state)
	if err != nil {
		return err
	}

	shown, err := elementShown(state, newStoryForm)
	if err != nil {
		return err
	}

	if shown {
		return nil
	}

	err = clickElement(state, []string{"", newStoryOpen})
	if err != nil {
		return err
	}

	return assertElementShown(state, []string{newStoryForm})
}

// createStoryUnderFeature is the whole act one clause names: the form, the
// title, the feature picked in it and the submit. args[0] is the role,
// discarded as clickElement's is.
func createStoryUnderFeature(state *State, args []string) error {
	err := openNewStoryForm(state, args)
	if err != nil {
		return err
	}

	err = fillElement(state, []string{"", newStoryTitle, args[1]})
	if err != nil {
		return err
	}

	err = pickFeature(state, newStoryForm, args[2])
	if err != nil {
		return err
	}

	return clickElement(state, []string{"", newStorySubmit})
}

// createStoryUnderCoinedFeature is the same act against a feature the product
// does not declare yet, whose id no other run can have produced.
func createStoryUnderCoinedFeature(state *State, args []string) error {
	err := openNewStoryForm(state, args)
	if err != nil {
		return err
	}

	err = fillElement(state, []string{"", newStoryTitle, args[1]})
	if err != nil {
		return err
	}

	feature := uniqueName(state, "feature")

	err = coinFeature(state, newStoryForm, feature)
	if err != nil {
		return err
	}

	state.NewFeature = feature

	return clickElement(state, []string{"", newStorySubmit})
}

// assertCreatedFileCount holds how many files matching the step's glob the tree
// gained since the snapshot to the number the step names. Polled: the save is
// the CLI's work and lands after the browser reports it.
func assertCreatedFileCount(state *State, args []string) error {
	want, glob := args[0], args[1]

	got, matched, err := await(readCreatedCount(state, glob), equals(want))
	if err != nil {
		return state.fail("listing the files matching %q: %w", glob, err)
	}

	if !matched {
		return state.fail("%s files matching %q were created, want exactly %s",
			got, glob, want)
	}

	return nil
}

// readCreatedCount counts the matching files the snapshot did not hold.
func readCreatedCount(state *State, glob string) func() (string, error) {
	return func() (string, error) {
		created, err := createdFiles(state, glob)
		if err != nil {
			return "", err
		}

		return strconv.Itoa(len(created)), nil
	}
}

// createdFiles are the tree's files matching the glob that the pre-edit snapshot
// did not hold — what "is created" means with no run to diff.
func createdFiles(state *State, glob string) ([]string, error) {
	if state.DocsBefore == nil {
		return nil, state.fail("%w", ErrNoDocumentSnapshot)
	}

	if state.Tree == nil {
		return nil, state.fail("%w", ErrNoProjectTree)
	}

	matches, err := filepath.Glob(filepath.Join(state.Tree.Dir, filepath.FromSlash(glob)))
	if err != nil {
		return nil, state.fail("globbing %q: %w", glob, err)
	}

	created := []string{}

	for _, match := range matches {
		rel, relErr := filepath.Rel(state.Tree.Dir, match)
		if relErr != nil {
			return nil, state.fail("placing %s in the project tree: %w", match, relErr)
		}

		path := filepath.ToSlash(rel)
		if _, held := state.DocsBefore[path]; !held {
			created = append(created, path)
		}
	}

	return created, nil
}

// createdStory is the one story document the form wrote, polled for the reason
// the created-count clause polls.
func createdStory(state *State) (string, error) {
	got, matched, err := await(readCreatedStories(state), isOnePath)
	if err != nil {
		return "", state.fail("looking for the story the form created: %w", err)
	}

	if !matched && got == "" {
		return "", state.fail("%w", ErrNoCreatedStory)
	}

	if !matched {
		return "", state.fail("%w: %s", ErrManyCreatedStories, got)
	}

	return got, nil
}

// readCreatedStories renders the created story documents as one reading, so a
// failure names every path it saw rather than only that there was not one.
func readCreatedStories(state *State) func() (string, error) {
	return func() (string, error) {
		created, err := createdFiles(state, storiesGlob)
		if err != nil {
			return "", err
		}

		return strings.Join(created, ", "), nil
	}
}

// isOnePath accepts a reading only once it names exactly one document.
func isOnePath(value string) bool {
	return value != "" && !strings.Contains(value, ", ")
}

// assertCreatedStoryFeature holds the created story's own document to carrying
// the feature the step names.
func assertCreatedStoryFeature(state *State, args []string) error {
	return createdStoryHasFeature(state, args[0])
}

// assertCreatedStoryCoinedFeature is that clause against the feature the When
// coined, which the scenario cannot name because no run knows it in advance.
func assertCreatedStoryCoinedFeature(state *State, _ []string) error {
	if state.NewFeature == "" {
		return state.fail("%w", ErrNoCoinedFeature)
	}

	return createdStoryHasFeature(state, state.NewFeature)
}

// createdStoryHasFeature is the shared body of the two clauses above.
func createdStoryHasFeature(state *State, feature string) error {
	relPath, err := createdStory(state)
	if err != nil {
		return err
	}

	return assertStoryFeature(state, relPath, feature)
}

// assertFileHoldsCoinedFeature holds the features file to declaring the coined
// feature with exactly the keys the step names and nothing else: a stub, not a
// second home for what the story already states.
func assertFileHoldsCoinedFeature(state *State, args []string) error {
	if state.NewFeature == "" {
		return state.fail("%w", ErrNoCoinedFeature)
	}

	relPath, want := args[0], strings.Join(keyNames(args[1]), ", ")

	got, matched, err := await(readFeatureKeys(state, relPath, state.NewFeature), equals(want))
	if err != nil {
		return state.fail("reading feature %q of %s: %w", state.NewFeature, relPath, err)
	}

	if !matched {
		return state.fail("feature %q of %s carries the keys %s, want exactly %s",
			state.NewFeature, relPath, got, want)
	}

	return nil
}
