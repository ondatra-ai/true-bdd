package steps

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// testTreeGlob is where a created test lives, which is what "the created test"
// names — the fix loop authors under the test root and nowhere else.
const testTreeGlob = "tests/**"

// registerRunArtifactSteps binds what the run left on disk: how many files it
// created, what a document now reads, and what the test it authored says.
func registerRunArtifactSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^exactly (\d+) files? matching "([^"]+)" (?:is|are) created$`,
		assertExactCreatedCount)
	suite.Step(`^the file "([^"]+)" matches (.+)$`, assertFileMatches)
	suite.Step(`^the created test matches (.+)$`, assertCreatedTestMatches)
	suite.Step(
		`^the description of acceptance criterion "([^"]+)" of "([^"]+)" matches (.+)$`,
		assertCriterionMatches)
	suite.Step(
		`^the description of acceptance criterion "([^"]+)" of "([^"]+)" `+
			`does not match (.+)$`,
		refuteCriterionMatches)
}

// assertExactCreatedCount serves the one wording both families of scenarios
// write: a form scenario snapshotted the writable documents and has no run to
// diff, a fix run has the tree's own baseline and took no snapshot.
func assertExactCreatedCount(state *State, args []string) error {
	if state.DocsBefore != nil {
		return assertCreatedFileCount(state, args)
	}

	return assertExactFilesCreated(state, args)
}

// assertExactFilesCreated is the exact-count twin of the at-least clause: the
// scenario that says ONE story file names a run that wrote a second one as wrong.
func assertExactFilesCreated(state *State, args []string) error {
	want, err := strconv.Atoi(args[0])
	if err != nil {
		return state.fail("the step names %q files, which is not a number: %w", args[0], err)
	}

	change, err := treeChanges(state)
	if err != nil {
		return err
	}

	matched, err := matchingPaths(state, change.Created, args[1])
	if err != nil {
		return err
	}

	if len(matched) != want {
		return state.fail("the run created %d file(s) matching %q, want exactly %d; "+
			"it created %s", len(matched), args[1], want, joinOrNone(change.Created))
	}

	return nil
}

// matchingPaths is every path of a list matching the glob a clause names.
func matchingPaths(state *State, paths []string, pattern string) ([]string, error) {
	matched := []string{}

	for _, path := range paths {
		hit, err := matchesGlob(pattern, path)
		if err != nil {
			return nil, state.fail("%w", err)
		}

		if hit {
			matched = append(matched, path)
		}
	}

	return matched, nil
}

// joinOrNone renders a path list, naming an empty one rather than reading blank.
func joinOrNone(paths []string) string {
	if len(paths) == 0 {
		return "no file"
	}

	return strings.Join(paths, ", ")
}

// assertFileMatches holds one document of the project tree to the step's
// regexp, which runs undelimited to the end of the line.
func assertFileMatches(state *State, args []string) error {
	relPath, expression := args[0], args[1]

	pattern, err := regexp.Compile(expression)
	if err != nil {
		return state.fail("the step's pattern %q does not compile: %w", expression, err)
	}

	raw, err := fixtureFile(state, relPath)
	if err != nil {
		return err
	}

	if !pattern.MatchString(raw) {
		return state.fail("%s does not match %s; it reads %s",
			relPath, pattern, snippetOf(raw))
	}

	return nil
}

// snippetOf renders the head of a document, so a failure carries what the file
// says without printing a whole registry into the test log.
func snippetOf(raw string) string {
	const limit = 400

	if len(raw) <= limit {
		return strconv.Quote(raw)
	}

	return strconv.Quote(raw[:limit]) + " …"
}

// assertCreatedTestMatches holds the test the run authored to the step's
// regexp: a fix loop that created a file under the test root but wrote the
// wrong scenario into it has converged on nothing.
func assertCreatedTestMatches(state *State, args []string) error {
	pattern, err := regexp.Compile(args[0])
	if err != nil {
		return state.fail("the step's pattern %q does not compile: %w", args[0], err)
	}

	change, err := treeChanges(state)
	if err != nil {
		return err
	}

	created, err := matchingPaths(state, change.Created, testTreeGlob)
	if err != nil {
		return err
	}

	if len(created) == 0 {
		return state.fail("the run created no file matching %q, want one matching %s; "+
			"it created %s", testTreeGlob, pattern, joinOrNone(change.Created))
	}

	return matchAnyFile(state, created, pattern)
}

// matchAnyFile passes as soon as one created test matches, and names every one
// it read when none does.
func matchAnyFile(state *State, paths []string, pattern *regexp.Regexp) error {
	for _, path := range paths {
		raw, err := fixtureFile(state, path)
		if err != nil {
			return err
		}

		if pattern.MatchString(raw) {
			return nil
		}
	}

	return state.fail("no created test matches %s; the run created %s",
		pattern, strings.Join(paths, ", "))
}

// assertCriterionMatches holds one acceptance criterion's description to the
// step's regexp, read off the document by the id the clause names.
func assertCriterionMatches(state *State, args []string) error {
	pattern, got, err := criterionReading(state, args)
	if err != nil {
		return err
	}

	if !pattern.MatchString(got) {
		return state.fail("acceptance criterion %s of %s reads %q, which does not match %s",
			args[0], args[1], got, pattern)
	}

	return nil
}

// refuteCriterionMatches is the negative twin: the wording a rewrite had to
// leave behind.
func refuteCriterionMatches(state *State, args []string) error {
	pattern, got, err := criterionReading(state, args)
	if err != nil {
		return err
	}

	if pattern.MatchString(got) {
		return state.fail("acceptance criterion %s of %s reads %q, which matches %s, "+
			"want a description that does not", args[0], args[1], got, pattern)
	}

	return nil
}

// criterionReading compiles the clause's pattern and reads the description of
// the criterion it names, which both twins open with.
func criterionReading(state *State, args []string) (*regexp.Regexp, string, error) {
	criterionID, relPath, expression := args[0], args[1], args[2]

	pattern, err := regexp.Compile(expression)
	if err != nil {
		return nil, "", state.fail("the step's pattern %q does not compile: %w",
			expression, err)
	}

	raw, err := fixtureFile(state, relPath)
	if err != nil {
		return nil, "", err
	}

	description, err := criterionDescription(raw, criterionID)
	if err != nil {
		return nil, "", state.fail("%s: %w", relPath, err)
	}

	return pattern, description, nil
}

// criterionDescription is the description of the criterion an id names, read
// off the same `- id: AC-…` blocks the modal oracle splits a document into.
func criterionDescription(raw, criterionID string) (string, error) {
	for _, block := range criterionBlocks(raw) {
		id, err := scalarField(block, "id")
		if err != nil {
			continue
		}

		if id == criterionID {
			return scalarField(block, "description")
		}
	}

	return "", fmt.Errorf("%w: %s", ErrNoCriterion, criterionID)
}
