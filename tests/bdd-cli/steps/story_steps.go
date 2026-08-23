package steps

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Story-shape assertions read a story a `us create … --fix` run authored:
// its id, criteria shape, and that a clause avoids forbidden vocabulary.
// Filenames are the fix loop's to choose, so steps resolve them by glob.
//	story:
//	  id: "99.1"
//	  as_a / i_want / so_that: …
//	  acceptance_criteria: [{id, description, steps}, …]

// storyCriterion is one acceptance criterion: id, description, and
// Given/When/Then steps decoded as single-key mappings
// (`given:`/`when:`/`then:`) holding step-node lists; wording stays undecoded.
type storyCriterion struct {
	ID          string                   `yaml:"id"`
	Description string                   `yaml:"description"`
	Steps       []map[string][]yaml.Node `yaml:"steps"`
}

// storyData is one story document reduced to what the story assertions
// need: id, acceptance criteria, and every top-level clause as a raw node
// so an arbitrarily-named one (as_a, i_want, so_that) can be looked up.
type storyData struct {
	ID       string
	Criteria []storyCriterion
	Clauses  map[string]yaml.Node
}

// readMatchedStory reads the one file a story glob resolved to, refusing a
// match that leaves the run directory through a symlink.
func (s *State) readMatchedStory(glob, match string) ([]byte, error) {
	full, err := s.containedMatch(glob, match)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(full)
	if err != nil {
		return nil, s.fail("reading story %q: %w", glob, err)
	}

	return data, nil
}

func loadStory(state *State, glob string) (storyData, error) {
	if state.Result == nil {
		return storyData{}, state.fail("%w", ErrNoRun)
	}

	matches, err := filepath.Glob(filepath.Join(state.Result.TmpDir, glob))
	if err != nil {
		return storyData{}, state.fail("story glob %q is not a valid pattern: %w", glob, err)
	}

	if len(matches) == 0 {
		return storyData{}, state.fail(
			"expected exactly one story matching %q, but the run created none", glob)
	}

	if len(matches) > 1 {
		return storyData{}, state.fail(
			"expected exactly one story matching %q, but %d exist: %v",
			glob, len(matches), relToTmp(state.Result.TmpDir, matches))
	}

	data, readErr := state.readMatchedStory(glob, matches[0])
	if readErr != nil {
		return storyData{}, readErr
	}

	var file struct {
		Story yaml.Node `yaml:"story"`
	}

	unmarshalErr := yaml.Unmarshal(data, &file)
	if unmarshalErr != nil {
		return storyData{}, state.fail("parsing story %q: %w", glob, unmarshalErr)
	}

	if file.Story.Kind == 0 {
		return storyData{}, state.fail("story %q has no top-level `story:` mapping", glob)
	}

	var typed struct {
		ID       string           `yaml:"id"`
		Criteria []storyCriterion `yaml:"acceptance_criteria"`
	}

	decodeErr := file.Story.Decode(&typed)
	if decodeErr != nil {
		return storyData{}, state.fail("decoding story %q: %w", glob, decodeErr)
	}

	clauses := map[string]yaml.Node{}

	decodeErr = file.Story.Decode(&clauses)
	if decodeErr != nil {
		return storyData{}, state.fail("decoding the clauses of story %q: %w", glob, decodeErr)
	}

	return storyData{ID: typed.ID, Criteria: typed.Criteria, Clauses: clauses}, nil
}

// missingStepKeywords returns which of given/when/then a criterion has no
// steps for, in that order. A keyword present with an empty list counts as
// missing: the criterion names the block and then says nothing in it.
func missingStepKeywords(criterion storyCriterion) []string {
	present := map[string]bool{"given": false, "when": false, "then": false}

	for _, entry := range criterion.Steps {
		for keyword, items := range entry {
			_, known := present[keyword]
			if known && len(items) > 0 {
				present[keyword] = true
			}
		}
	}

	var missing []string

	for _, keyword := range []string{"given", "when", "then"} {
		if !present[keyword] {
			missing = append(missing, keyword)
		}
	}

	return missing
}

// criterionLabel names a criterion in a failure: its own id when it has
// one, otherwise its 1-based position, so a story whose criteria are
// unlabelled still produces a message a reader can act on.
func criterionLabel(criterion storyCriterion, index int) string {
	if criterion.ID != "" {
		return criterion.ID
	}

	return fmt.Sprintf("#%d", index+1)
}

// findCriterion returns the criterion with the given id, or nil. Returned
// by pointer into the story's own slice so a caller reads the criterion the
// story actually holds rather than a copy.
func findCriterion(story storyData, criterionID string) *storyCriterion {
	for index := range story.Criteria {
		if story.Criteria[index].ID == criterionID {
			return &story.Criteria[index]
		}
	}

	return nil
}

// criterionStepTexts flattens every step of a criterion to its wording,
// whichever of the two shapes each was written in. See collectStepText for
// why a mapping node's keys are skipped and its values are not.
func criterionStepTexts(criterion *storyCriterion) []string {
	var steps []string

	for _, entry := range criterion.Steps {
		for _, nodes := range entry {
			for index := range nodes {
				collectStepText(&nodes[index], &steps)
			}
		}
	}

	return steps
}

// relToTmp rewrites absolute match paths back to tmpdir-relative ones so a
// failure names the stories the way the scenario did, not by their
// per-run absolute path.
func relToTmp(tmpDir string, paths []string) []string {
	rels := make([]string, 0, len(paths))

	for _, path := range paths {
		rel, err := filepath.Rel(tmpDir, path)
		if err != nil {
			rels = append(rels, path)

			continue
		}

		rels = append(rels, rel)
	}

	return rels
}

// assertStoryID pins the id the story document carries. The glob and the
// expected id are both capture groups so one definition serves every
// scenario naming a different story or id.
func assertStoryID(state *State, args []string) error {
	story, err := loadStory(state, args[0])
	if err != nil {
		return err
	}

	want := args[1]

	if story.ID != want {
		return state.fail("expected story %q to have id %q, but it has %q", args[0], want, story.ID)
	}

	return nil
}

// assertStoryCriteriaCount pins a lower bound on acceptance criteria —
// "at least", since the fix loop may add or polish criteria and the
// scenario pins only the floor it must clear.
func assertStoryCriteriaCount(state *State, args []string) error {
	story, err := loadStory(state, args[0])
	if err != nil {
		return err
	}

	want, err := strconv.Atoi(args[1])
	if err != nil {
		return state.fail("acceptance-criteria count %q is not a number: %w", args[1], err)
	}

	if got := len(story.Criteria); got < want {
		return state.fail(
			"expected story %q to have at least %d acceptance criteria, but it has %d",
			args[0], want, got)
	}

	return nil
}

// assertStoryCriteriaExactCount pins the EXACT criteria count — the
// tighter twin of assertStoryCriteriaCount's "at least", proving a fix
// loop that rewrote the story neither dropped nor added one.
func assertStoryCriteriaExactCount(state *State, args []string) error {
	story, err := loadStory(state, args[0])
	if err != nil {
		return err
	}

	want, err := strconv.Atoi(args[1])
	if err != nil {
		return state.fail("acceptance-criteria count %q is not a number: %w", args[1], err)
	}

	if got := len(story.Criteria); got != want {
		return state.fail(
			"expected story %q to have exactly %d acceptance criteria, but it has %d",
			args[0], want, got)
	}

	return nil
}

// assertStoryCriteriaWellFormed pins that EVERY criterion has an id
// matching a regexp and a non-empty description; a story with no
// criteria fails rather than passing vacuously.
func assertStoryCriteriaWellFormed(state *State, args []string) error {
	story, err := loadStory(state, args[0])
	if err != nil {
		return err
	}

	idPattern, err := regexp.Compile(args[1])
	if err != nil {
		return state.fail("criterion-id pattern %q does not compile: %w", args[1], err)
	}

	if len(story.Criteria) == 0 {
		return state.fail(
			"expected every acceptance criterion in story %q to have an id matching %q and a "+
				"non-empty description, but the story has no criteria", args[0], args[1])
	}

	var problems []string

	for index, criterion := range story.Criteria {
		if !idPattern.MatchString(criterion.ID) {
			problems = append(problems, fmt.Sprintf(
				"criterion %d has id %q, which does not match %q", index+1, criterion.ID, args[1]))
		}

		if strings.TrimSpace(criterion.Description) == "" {
			problems = append(problems, fmt.Sprintf(
				"criterion %d (id %q) has an empty description", index+1, criterion.ID))
		}
	}

	if len(problems) > 0 {
		return state.fail(
			"expected every acceptance criterion in story %q to have an id matching %q and a "+
				"non-empty description, but: %s", args[0], args[1], strings.Join(problems, "; "))
	}

	return nil
}

// assertStoryClauseNoMatch pins that a named clause (as_a, i_want,
// so_that) does NOT match a forbidden regexp, e.g. driving an
// implementation term out of so_that. Pattern is undelimited, like `stdout matches`.
func assertStoryClauseNoMatch(state *State, args []string) error {
	clause := args[0]

	story, err := loadStory(state, args[1])
	if err != nil {
		return err
	}

	pattern, err := regexp.Compile(args[2])
	if err != nil {
		return state.fail("clause-match pattern %q does not compile: %w", args[2], err)
	}

	node, ok := story.Clauses[clause]
	if !ok {
		return state.fail("expected story %q to carry a %q clause, but it has none", args[1], clause)
	}

	var value string

	decodeErr := node.Decode(&value)
	if decodeErr != nil {
		return state.fail("reading the %q clause of story %q: %w", clause, args[1], decodeErr)
	}

	if pattern.MatchString(value) {
		return state.fail(
			"expected the %q clause of story %q not to match %q, but it did:\n%s",
			clause, args[1], args[2], value)
	}

	return nil
}

// assertStoryCriteriaHaveSteps pins that EVERY criterion carries a
// non-empty given, when and then block; a story with no criteria fails
// rather than passing vacuously.
func assertStoryCriteriaHaveSteps(state *State, args []string) error {
	story, err := loadStory(state, args[0])
	if err != nil {
		return err
	}

	if len(story.Criteria) == 0 {
		return state.fail(
			"expected every acceptance criterion in story %q to have given, when and then steps, "+
				"but the story has no criteria", args[0])
	}

	var problems []string

	for index, criterion := range story.Criteria {
		missing := missingStepKeywords(criterion)
		if len(missing) == 0 {
			continue
		}

		problems = append(problems, fmt.Sprintf("criterion %s is missing %s step(s)",
			criterionLabel(criterion, index), strings.Join(missing, ", ")))
	}

	if len(problems) > 0 {
		return state.fail(
			"expected every acceptance criterion in story %q to have given, when and then steps, "+
				"but: %s", args[0], strings.Join(problems, "; "))
	}

	return nil
}

// assertStoryCriterionDescNoMatch is the per-criterion twin of
// assertStoryClauseNoMatch: a NAMED criterion's description must NOT
// match a forbidden regexp. Resolved by id; no such criterion fails.
func assertStoryCriterionDescNoMatch(state *State, args []string) error {
	criterionID := args[0]

	story, err := loadStory(state, args[1])
	if err != nil {
		return err
	}

	pattern, err := regexp.Compile(args[2])
	if err != nil {
		return state.fail("description-match pattern %q does not compile: %w", args[2], err)
	}

	found := findCriterion(story, criterionID)
	if found == nil {
		return state.fail(
			"expected story %q to carry an acceptance criterion %q, but it has none",
			args[1], criterionID)
	}

	if pattern.MatchString(found.Description) {
		return state.fail(
			"expected the description of acceptance criterion %q of story %q not to match %q, "+
				"but it did:\n%s", criterionID, args[1], args[2], found.Description)
	}

	return nil
}

// assertStoryCriterionDescMatch is the positive twin of
// assertStoryCriterionDescNoMatch: a NAMED criterion's description MUST
// match a required regexp (e.g. a "must"/"should" claim).
func assertStoryCriterionDescMatch(state *State, args []string) error {
	criterionID := args[0]

	story, err := loadStory(state, args[1])
	if err != nil {
		return err
	}

	pattern, err := regexp.Compile(args[2])
	if err != nil {
		return state.fail("description-match pattern %q does not compile: %w", args[2], err)
	}

	found := findCriterion(story, criterionID)
	if found == nil {
		return state.fail(
			"expected story %q to carry an acceptance criterion %q, but it has none",
			args[1], criterionID)
	}

	if !pattern.MatchString(found.Description) {
		return state.fail(
			"expected the description of acceptance criterion %q of story %q to match %q, "+
				"but it did not:\n%s", criterionID, args[1], args[2], found.Description)
	}

	return nil
}

// assertStoryCriterionStepsNoMatch is the step-text twin of
// assertStoryCriterionDescNoMatch: NONE of a NAMED criterion's step texts
// may match a forbidden regexp. A criterion with no steps fails.
func assertStoryCriterionStepsNoMatch(state *State, args []string) error {
	criterionID := args[0]

	story, err := loadStory(state, args[1])
	if err != nil {
		return err
	}

	pattern, err := regexp.Compile(args[2])
	if err != nil {
		return state.fail("step-match pattern %q does not compile: %w", args[2], err)
	}

	found := findCriterion(story, criterionID)
	if found == nil {
		return state.fail(
			"expected story %q to carry an acceptance criterion %q, but it has none",
			args[1], criterionID)
	}

	steps := criterionStepTexts(found)
	if len(steps) == 0 {
		return state.fail(
			"expected acceptance criterion %q of story %q to have steps not matching %q, "+
				"but it has no steps", criterionID, args[1], args[2])
	}

	var offenders []string

	for _, step := range steps {
		if pattern.MatchString(step) {
			offenders = append(offenders, step)
		}
	}

	if len(offenders) > 0 {
		return state.fail(
			"expected no step of acceptance criterion %q of story %q to match %q, but %d did:\n%s",
			criterionID, args[1], args[2], len(offenders), strings.Join(offenders, "\n"))
	}

	return nil
}

// collectStepText gathers a story step node's text, descending into
// `and:`-style continuation mappings and lists, so a step's wording is
// read whether written as a bare string or a continuation.
func collectStepText(node *yaml.Node, out *[]string) {
	switch node.Kind {
	case yaml.ScalarNode:
		if strings.TrimSpace(node.Value) != "" {
			*out = append(*out, node.Value)
		}
	case yaml.MappingNode:
		// content is key,val,key,val,… — only the values carry step text.
		for i := 1; i < len(node.Content); i += 2 {
			collectStepText(node.Content[i], out)
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			collectStepText(child, out)
		}
	case yaml.DocumentNode, yaml.AliasNode:
		// Neither can appear inside a decoded steps value: a document node
		// is the file's root, and the loader refuses aliases outright.
		return
	}
}
