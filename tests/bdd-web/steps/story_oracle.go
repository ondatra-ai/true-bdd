package steps

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoCriterion is returned when the oracle document declares fewer
// acceptance criteria than a clause names.
var ErrNoCriterion = errors.New("the document declares no such acceptance criterion")

// ErrNoStepOfKind is returned when the story declares no step of the kind a
// clause names.
var ErrNoStepOfKind = errors.New("the story declares no step of that kind")

// registerStoryOracleSteps binds the clauses that read a modal panel against
// the document it renders: the story file itself, or what an epic declares for
// a story no file holds.
func registerStoryOracleSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^(`+selectorPattern+`) holds the statement of "([^"]+)"$`,
		assertStatementOfFile)
	suite.Step(`^(`+selectorPattern+`) holds acceptance criterion (\d+) of "([^"]+)"$`,
		assertCriterionOfFile)
	suite.Step(`^(`+selectorPattern+`) holds the first "([^"]+)" step of "([^"]+)"$`,
		assertFirstStepOfFile)
	suite.Step(`^(`+selectorPattern+`) holds the bytes of "([^"]+)"$`, assertBytesOfFile)
	suite.Step(
		`^(`+selectorPattern+`) holds the statement declared for story "([^"]+)" in "([^"]+)"$`,
		assertDeclaredStatement)
	suite.Step(
		`^(`+selectorPattern+`) holds acceptance criterion (\d+) declared for story "([^"]+)" `+
			`in "([^"]+)"$`,
		assertDeclaredCriterion)
}

// statementFields are the three scalars a story's statement is made of.
func statementFields() []string {
	return []string{"as_a", "i_want", "so_that"}
}

// assertStatementOfFile holds the panel to the statement the story file states.
func assertStatementOfFile(state *State, args []string) error {
	relPath := args[1]

	raw, err := fixtureFile(state, relPath)
	if err != nil {
		return err
	}

	fragments, err := statementOf(state, raw, relPath)
	if err != nil {
		return err
	}

	return assertHolds(state, args[0], "the statement of "+relPath, fragments)
}

// assertDeclaredStatement is the same clause against what the EPIC declares —
// what a story with no file of its own is reviewed on.
func assertDeclaredStatement(state *State, args []string) error {
	storyID, relPath := args[1], args[2]

	block, err := declaredStory(state, relPath, storyID)
	if err != nil {
		return err
	}

	fragments, err := statementOf(state, block, relPath)
	if err != nil {
		return err
	}

	return assertHolds(state, args[0],
		fmt.Sprintf("the statement declared for story %s in %s", storyID, relPath),
		fragments)
}

// assertCriterionOfFile holds the panel to one acceptance criterion of the
// story file, one-based as the step writes it.
func assertCriterionOfFile(state *State, args []string) error {
	ordinal, err := ordinalOf(state, args[1])
	if err != nil {
		return err
	}

	relPath := args[2]

	raw, err := fixtureFile(state, relPath)
	if err != nil {
		return err
	}

	want, err := criterionOf(raw, ordinal)
	if err != nil {
		return state.fail("%s: %w", relPath, err)
	}

	return assertHolds(state, args[0],
		fmt.Sprintf("acceptance criterion %d of %s", ordinal, relPath), []string{want})
}

// assertDeclaredCriterion is that clause against the epic's declaration.
func assertDeclaredCriterion(state *State, args []string) error {
	ordinal, err := ordinalOf(state, args[1])
	if err != nil {
		return err
	}

	storyID, relPath := args[2], args[3]

	block, err := declaredStory(state, relPath, storyID)
	if err != nil {
		return err
	}

	want, err := criterionOf(block, ordinal)
	if err != nil {
		return state.fail("story %s of %s: %w", storyID, relPath, err)
	}

	return assertHolds(state, args[0],
		fmt.Sprintf("acceptance criterion %d declared for story %s in %s",
			ordinal, storyID, relPath), []string{want})
}

// assertFirstStepOfFile holds the panel to the first step of one kind the
// story declares — the clause that proves every kind is rendered, not only
// given/when/then.
func assertFirstStepOfFile(state *State, args []string) error {
	kind, relPath := args[1], args[2]

	raw, err := fixtureFile(state, relPath)
	if err != nil {
		return err
	}

	want, err := firstStepOf(raw, kind)
	if err != nil {
		return state.fail("%s: %w", relPath, err)
	}

	return assertHolds(state, args[0],
		fmt.Sprintf("the first %q step of %s", kind, relPath), []string{want})
}

// assertBytesOfFile holds the raw panel to the file's own bytes: a RENDERING
// of the document is exactly what this clause exists to refuse.
func assertBytesOfFile(state *State, args []string) error {
	relPath := args[1]

	raw, err := fixtureFile(state, relPath)
	if err != nil {
		return err
	}

	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	want := strings.TrimSpace(raw)

	got, matched, err := await(readInnerText(locator), equals(want))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s reads %q, want the bytes of %s, %q", sel, got, relPath, want)
	}

	return nil
}

// assertHolds is where every clause in this file ends: the rendered text must
// carry each fragment the oracle states, so a rendering that reflows or labels
// it still passes and one that drops it cannot.
func assertHolds(state *State, text, subject string, fragments []string) error {
	sel, locator, err := locateStep(state, text)
	if err != nil {
		return err
	}

	got, matched, err := await(readInnerText(locator), containsAll(fragments))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s reads %q, which does not carry %s: %s",
			sel, got, subject, strings.Join(fragments, " | "))
	}

	return nil
}

// containsAll accepts a reading only once every fragment is in it.
func containsAll(fragments []string) func(string) bool {
	return func(value string) bool {
		for _, fragment := range fragments {
			if !strings.Contains(value, fragment) {
				return false
			}
		}

		return true
	}
}

// ordinalOf reads the one-based number a criterion clause names.
func ordinalOf(state *State, raw string) (int, error) {
	ordinal, err := strconv.Atoi(raw)
	if err != nil {
		return 0, state.fail("the step names criterion %q, which is not a number: %w",
			raw, err)
	}

	return ordinal, nil
}

// declaredStory is the region of an epic document declaring one story.
func declaredStory(state *State, relPath, storyID string) (string, error) {
	raw, err := fixtureFile(state, relPath)
	if err != nil {
		return "", err
	}

	block, err := storyBlock(raw, storyID)
	if err != nil {
		return "", state.fail("%s: %w", relPath, err)
	}

	return block, nil
}

// statementOf reads the scalars a statement is made of out of the region of
// YAML that declares it — the story file's own, or an epic's story block.
func statementOf(state *State, region, source string) ([]string, error) {
	values := []string{}

	for _, field := range statementFields() {
		value, err := scalarField(region, field)
		if err != nil {
			return nil, state.fail("%s: %w", source, err)
		}

		values = append(values, value)
	}

	return values, nil
}

// criterionOf is the description of the Nth acceptance criterion declared in a
// region, one-based.
func criterionOf(region string, ordinal int) (string, error) {
	blocks := criterionBlocks(region)
	if ordinal < 1 || ordinal > len(blocks) {
		return "", fmt.Errorf("%w: %d of %d declared", ErrNoCriterion, ordinal, len(blocks))
	}

	return scalarField(blocks[ordinal-1], "description")
}

// criterionBlocks splits a region at each `- id: AC-…` line, so the Nth block
// is the Nth criterion whatever it declares under it.
func criterionBlocks(region string) []string {
	lines := strings.Split(region, "\n")
	opener := regexp.MustCompile(`^\s*-\s+id:\s*["']?AC-`)
	starts := []int{}

	for index, line := range lines {
		if opener.MatchString(line) {
			starts = append(starts, index)
		}
	}

	blocks := make([]string, 0, len(starts))

	for position, start := range starts {
		end := len(lines)
		if position+1 < len(starts) {
			end = starts[position+1]
		}

		blocks = append(blocks, strings.Join(lines[start:end], "\n"))
	}

	return blocks
}

// firstStepOf is the first step of one kind a story declares. A keyword list
// item (`- and: text`) carries its text on the line; a block keyword
// (`- given:`) opens a list whose first item is the step.
func firstStepOf(raw, kind string) (string, error) {
	lines := strings.Split(raw, "\n")
	opener := regexp.MustCompile(`^\s*-?\s*` + regexp.QuoteMeta(kind) + `:\s*(.*)$`)

	for index, line := range lines {
		match := opener.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		if strings.TrimSpace(match[1]) != "" {
			return unquote(match[1]), nil
		}

		text, found := firstListItem(lines[index+1:])
		if found {
			return text, nil
		}
	}

	return "", fmt.Errorf("%w: %s", ErrNoStepOfKind, kind)
}

// firstListItem is the text of the first `- …` item of the list that follows,
// which is the step a block keyword opened.
func firstListItem(lines []string) (string, bool) {
	item := regexp.MustCompile(`^\s*-\s+(.+?)\s*$`)

	for _, line := range lines {
		match := item.FindStringSubmatch(line)
		if match != nil {
			return unquote(match[1]), true
		}
	}

	return "", false
}
