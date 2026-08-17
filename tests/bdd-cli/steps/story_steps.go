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

// Story-shape assertions read a story document a `us create … --fix` run
// authored and check its shape rather than merely that a file appeared:
// the id it carries, how many acceptance criteria it has, that each
// criterion is well-formed, and that a named narrative clause avoids a
// forbidden vocabulary. These are claims no file-effect step can make —
// "the file was created" cannot see that the so_that clause still speaks
// of an implementation term the fix loop was meant to drive out.
//
// The story's filename is the fix loop's to choose (`<id>-<slug>.yaml`),
// so every one of these steps names the file by GLOB and resolves it to
// the single matching file on disk under the run's tmpdir. Zero matches
// (the run wrote no such story) and more than one (an ambiguous glob) are
// each their own failure rather than a silently-picked file.
//
// The document nests everything under a top-level `story:` key:
//
//	story:
//	  id: "99.1"
//	  as_a / i_want / so_that: …
//	  acceptance_criteria: [{id, description, steps}, …]
//
// The glob, the id, the count, the criterion-id regexp, the clause name
// and the forbidden pattern are all capture groups, so one definition
// each serves every scenario naming a different target.

// storyCriterion is the slice of one acceptance criterion these
// assertions read: its id, its description, and its Given/When/Then steps.
// The steps decode as a sequence of single-key mappings
// (`given:`/`when:`/`then:`), each holding a list of step nodes — exactly
// what a "has given, when and then steps" assertion needs to see present
// and non-empty without reading the step text itself. The individual step
// wording stays undecoded: these assertions ask about a criterion's shape,
// not its behaviour.
type storyCriterion struct {
	ID          string                   `yaml:"id"`
	Description string                   `yaml:"description"`
	Steps       []map[string][]yaml.Node `yaml:"steps"`
}

// storyData is one story document reduced to what the story assertions
// need: its id, its acceptance criteria, and every top-level clause kept
// as a raw node so an arbitrarily-named one (as_a, i_want, so_that) can be
// looked up by the clause the scenario names.
type storyData struct {
	ID       string
	Criteria []storyCriterion
	Clauses  map[string]yaml.Node
}

// loadStory resolves a story glob to the single file it names on disk
// under the run's tmpdir and parses it. The filename is the fix loop's to
// choose, so the scenario can only name the story by glob; a glob that
// matches nothing or more than one file is named in its own failure
// rather than yielding a silently-picked or empty story.
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

// assertStoryCriteriaCount pins a lower bound on how many acceptance
// criteria the story document carries — "at least", because the fix loop
// may polish or add criteria and a scenario pins the floor it must clear,
// not an exact tally. The glob and the count are both capture groups.
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

// assertStoryCriteriaExactCount pins the EXACT number of acceptance
// criteria the story document carries — the tighter twin of
// assertStoryCriteriaCount's "at least". A scenario uses it to prove a fix
// loop that rewrites the whole story neither dropped nor added a criterion:
// it must still have precisely as many as the seed. The glob and the count
// are both capture groups.
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

// assertStoryCriteriaWellFormed pins that EVERY acceptance criterion of the
// story is well-formed: its id matches a regexp and its description is
// non-empty. A story with no criteria fails rather than passing vacuously.
// The glob and the id regexp are both capture groups; the regexp runs
// undelimited up to ` and a non-empty description`, so an anchored pattern
// like `^AC-\d+$` needs no escaping.
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

// assertStoryClauseNoMatch pins that a named narrative clause of the story
// (as_a, i_want, so_that) does NOT match a forbidden regexp — how a
// scenario proves the fix loop drove an implementation term out of the
// so_that clause until it states a user outcome. The clause name, the glob
// and the pattern are all capture groups; the pattern runs undelimited to
// the end of the line, like `stdout matches`, so a regexp carrying its own
// alternation and word boundaries needs no escaping.
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

// assertStoryCriteriaHaveSteps pins that EVERY acceptance criterion of the
// story carries Given, When and Then steps — how a scenario proves a fix
// loop that rewrites a story preserved each criterion's executable shape
// rather than leaving a description with no steps behind it. A criterion is
// satisfied when its `steps` sequence holds a non-empty `given`, a non-empty
// `when` and a non-empty `then` block; a story with no criteria fails rather
// than passing vacuously. The glob is a capture group so one definition
// serves every scenario naming a different story.
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

// assertStoryCriterionDescNoMatch pins that the description of a NAMED
// acceptance criterion does NOT match a forbidden regexp — how a scenario
// proves the fix loop drove a vague qualifier out of one criterion's
// description until it reads as a measurable claim. It is the per-criterion
// twin of assertStoryClauseNoMatch, which speaks of a top-level narrative
// clause. The criterion is resolved by id; a story that carries no such
// criterion fails rather than passing vacuously. The criterion id, the glob
// and the pattern are all capture groups; the pattern runs undelimited to
// the end of the line, like `stdout matches`, so a regexp carrying its own
// alternation and word boundaries needs no escaping.
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

// assertStoryCriterionDescMatch pins that the description of a NAMED
// acceptance criterion DOES match a required regexp — the positive twin of
// assertStoryCriterionDescNoMatch. It is how a scenario proves the fix loop
// drove a criterion's description INTO a required shape (a rule-based
// "must"/"should" claim), where the negative twin only proves it was driven
// away from a forbidden one. The criterion is resolved by id; a story that
// carries no such criterion fails rather than passing vacuously. The
// criterion id, the glob and the pattern are all capture groups; the pattern
// runs undelimited to the end of the line, like `stdout matches`, so a regexp
// carrying its own alternation and word boundaries needs no escaping.
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

// assertStoryCriterionStepsNoMatch pins that NONE of the Given/When/Then step
// texts of a NAMED acceptance criterion match a forbidden regexp — how a
// scenario proves the fix loop drove a forbidden action verb out of a
// criterion's steps until each step names an observable action. It is the
// step-text twin of assertStoryCriterionDescNoMatch, which speaks only of a
// criterion's description. The criterion is resolved by id; a story that
// carries no such criterion, or a criterion carrying no steps, fails rather
// than passing vacuously. The criterion id, the glob and the pattern are all
// capture groups; the pattern runs undelimited to the end of the line, like
// `stdout matches`, so a regexp carrying its own alternation and word
// boundaries needs no escaping.
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

// collectStepText gathers the human-readable text of a story step node,
// descending into the single-key `and:`-style continuation mappings and the
// lists a step list may hold: a scalar node is the step text itself, and a
// mapping node's values carry the continuation text while its keys
// (given/when/then/and) do not. It is what lets a steps assertion read the
// wording of every step of a criterion whether it was written as a bare
// string or an `and:` continuation, so matching a forbidden verb never
// silently skips a step because of how it was phrased.
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
