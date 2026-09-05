package steps

import (
	"sort"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// registerRunFactSteps binds what a dispatched run reads back as, and what its
// event window says about how it got there.
func registerRunFactSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^(`+runRefPattern+`) has command "([^"]*)"$`, assertRunCommand)
	suite.Step(`^(`+runRefPattern+`) has story "([^"]*)"$`, assertRunStory)
	suite.Step(`^(`+runRefPattern+`) has fix unset$`, assertRunFixUnset)
	suite.Step(`^(`+runRefPattern+`) was applied at least once$`, assertRunApplied)
	suite.Step(`^(`+runRefPattern+`) recorded events beyond the ones it had at the prompt$`,
		assertEventsGrew)
	suite.Step(`^(`+runRefPattern+`)'s event sequence numbers are unique$`, assertSeqUnique)
	suite.Step(`^(`+runRefPattern+`) published exactly (\d+) prompts?$`, assertPromptCount)
	suite.Step(`^(`+runRefPattern+`)'s prompts began ("[^"]*"(?:, "[^"]*")*)$`, assertPromptOrder)
	suite.Step(`^every sequence number recorded before the restart appears exactly once$`,
		assertPreRestartSeqsIntact)
	suite.Step(`^the second choice prompt carries an id different from the first `+
		`and from the freetext one$`, assertSecondChoiceDistinct)
	// The three clauses the suite already owns for a quoted label. Disjoint
	// patterns rather than widened ones, so a step matches exactly one definition.
	suite.Step(`^the dispatched run has fix set$`, assertDispatchedRunFix)
	suite.Step(`^the dispatched run reaches a terminal state$`, assertDispatchedRunTerminal)
	suite.Step(`^the dispatched run has outcome "([^"]*)"$`, assertDispatchedRunOutcome)
}

// runDetailOf reads one labelled run's detail — the read every clause below grades.
func runDetailOf(state *State, label string) (*runDetail, error) {
	path, err := runPath(state, label)
	if err != nil {
		return nil, err
	}

	return readRun(state, path)
}

// runFact resolves the run a clause names and reads it.
func runFact(state *State, ref string) (string, *runDetail, error) {
	label, err := runLabelOf(state, ref)
	if err != nil {
		return "", nil, err
	}

	detail, err := runDetailOf(state, label)

	return label, detail, err
}

// assertRunCommand holds the run to the command it was dispatched as: a row's
// action that sent the wrong verb is a reader who asked for something else.
func assertRunCommand(state *State, args []string) error {
	label, detail, err := runFact(state, args[0])
	if err != nil {
		return err
	}

	if detail.Command != args[1] {
		return state.fail("run %q has command %q, want %q", label, detail.Command, args[1])
	}

	return nil
}

// assertRunStory holds it to the story it names.
func assertRunStory(state *State, args []string) error {
	label, detail, err := runFact(state, args[0])
	if err != nil {
		return err
	}

	if detail.StoryID == nil {
		return state.fail("run %q names no story, want %q", label, args[1])
	}

	if *detail.StoryID != args[1] {
		return state.fail("run %q has story %q, want %q", label, *detail.StoryID, args[1])
	}

	return nil
}

// assertRunFixUnset is the negative of the suite's `has fix set`: a run the reader
// never armed must not read back as a fix run.
func assertRunFixUnset(state *State, args []string) error {
	label, detail, err := runFact(state, args[0])
	if err != nil {
		return err
	}

	if detail.Fix {
		return state.fail("run %q reads back with fix set, want it unset", label)
	}

	return nil
}

// assertRunApplied holds the run to having taken the apply choice: a fix loop that
// converged without ever applying converged on nothing.
func assertRunApplied(state *State, args []string) error {
	label, err := runLabelOf(state, args[0])
	if err != nil {
		return err
	}

	if state.Applies[label] == 0 {
		return state.fail("run %q never took the apply choice, want it applied at least once",
			label)
	}

	return nil
}

// assertEventsGrew holds the run to having recorded more than it had while it was
// blocked: a window that stopped at the prompt says nothing about how it ended.
func assertEventsGrew(state *State, args []string) error {
	label, detail, err := runFact(state, args[0])
	if err != nil {
		return err
	}

	before, seen := state.PromptEvents[label]
	if !seen {
		return state.fail("%w: run %q", ErrNoPrompt, label)
	}

	if len(detail.Events) <= len(before) {
		return state.fail("run %q holds %d event(s), want more than the %d it had at the prompt",
			label, len(detail.Events), len(before))
	}

	return nil
}

// assertSeqUnique holds the window to one record per sequence number: a duplicated
// seq is a reader shown the same line twice.
func assertSeqUnique(state *State, args []string) error {
	label, detail, err := runFact(state, args[0])
	if err != nil {
		return err
	}

	counts := seqCounts(detail.Events)

	for _, seq := range sortedSeqs(counts) {
		if counts[seq] > 1 {
			return state.fail("run %q records sequence number %d %d times, want once",
				label, seq, counts[seq])
		}
	}

	return nil
}

// assertPreRestartSeqsIntact holds every sequence number the run had while it was
// blocked to surviving exactly once: a window that dropped or doubled one shows a
// reader a different run than happened.
func assertPreRestartSeqsIntact(state *State, _ []string) error {
	label := state.Prompted
	if label == "" {
		return state.fail("%w", ErrNoPrompt)
	}

	before, seen := state.PromptEvents[label]
	if !seen {
		return state.fail("%w: run %q", ErrNoPrompt, label)
	}

	detail, err := runDetailOf(state, label)
	if err != nil {
		return err
	}

	counts := seqCounts(detail.Events)

	for _, seq := range before {
		if counts[seq] != 1 {
			return state.fail("run %q records sequence number %d %d times after the restart, "+
				"want exactly once", label, seq, counts[seq])
		}
	}

	return nil
}

// assertPromptCount holds the run to the number of DISTINCT prompts the step
// names, counted off the prompt ids its own event window carries.
func assertPromptCount(state *State, args []string) error {
	label, detail, err := runFact(state, args[0])
	if err != nil {
		return err
	}

	want, err := strconv.Atoi(args[1])
	if err != nil {
		return state.fail("the step names %q prompts, which is not a number: %w", args[1], err)
	}

	ids := map[string]bool{}

	for _, event := range detail.Events {
		if event.PromptID != "" {
			ids[event.PromptID] = true
		}
	}

	if len(ids) != want {
		return state.fail("run %q published %d prompt(s), want %d; it recorded %s",
			label, len(ids), want, eventTypes(detail.Events))
	}

	return nil
}

// assertPromptOrder holds the prompts the scenario answered to the opening
// sequence the step lists: the refine loop's shape is choice, freetext, choice,
// and a loop that skipped one is a loop that never asked.
func assertPromptOrder(state *State, args []string) error {
	label, err := runLabelOf(state, args[0])
	if err != nil {
		return err
	}

	want := quotedList(args[1])
	history := state.PromptHistory[label]

	if len(history) < len(want) {
		return state.fail("run %q published %s, want it to begin %s",
			label, promptKindList(history), strings.Join(want, ", "))
	}

	for index, kind := range want {
		if history[index].Kind != kind {
			return state.fail("run %q's prompt %d is %q, want %q; it published %s",
				label, index+1, history[index].Kind, kind, promptKindList(history))
		}
	}

	return nil
}

// assertSecondChoiceDistinct holds the refine loop's second question to being a
// NEW prompt: one that re-published an id already answered would be answered twice
// by the reader's single click.
func assertSecondChoiceDistinct(state *State, _ []string) error {
	label := state.Prompted
	if label == "" {
		return state.fail("%w", ErrNoPrompt)
	}

	history := state.PromptHistory[label]
	choices := idsOfKind(history, choiceKind)
	freetexts := idsOfKind(history, freetextKind)

	if len(choices) < 2 || len(freetexts) == 0 {
		return state.fail("run %q published %s, want two choice prompts and a freetext one",
			label, promptKindList(history))
	}

	if choices[1] == choices[0] || choices[1] == freetexts[0] {
		return state.fail("run %q's second choice prompt carries id %q, which its first "+
			"choice (%q) or its freetext (%q) already used",
			label, choices[1], choices[0], freetexts[0])
	}

	return nil
}

// assertDispatchedRunFix, assertDispatchedRunTerminal and assertDispatchedRunOutcome
// resolve the page-dispatched run and delegate to the suite's own clause, so the
// two phrasings are graded by one implementation.
func assertDispatchedRunFix(state *State, _ []string) error {
	label, err := runLabelOf(state, dispatchedLabel)
	if err != nil {
		return err
	}

	return assertRunFix(state, []string{label})
}

func assertDispatchedRunTerminal(state *State, _ []string) error {
	label, err := runLabelOf(state, dispatchedLabel)
	if err != nil {
		return err
	}

	return assertRunTerminal(state, []string{label})
}

func assertDispatchedRunOutcome(state *State, args []string) error {
	label, err := runLabelOf(state, dispatchedLabel)
	if err != nil {
		return err
	}

	return assertRunOutcome(state, []string{label, args[0]})
}

// seqCounts counts how often each sequence number appears in an event window.
func seqCounts(events []runEvent) map[int]int {
	counts := map[int]int{}

	for _, event := range events {
		counts[event.Seq]++
	}

	return counts
}

// sortedSeqs orders them, so a failure names the FIRST offender rather than
// whichever one map iteration reached first.
func sortedSeqs(counts map[int]int) []int {
	seqs := make([]int, 0, len(counts))

	for seq := range counts {
		seqs = append(seqs, seq)
	}

	sort.Ints(seqs)

	return seqs
}

// eventTypes renders the window's types, so a count failure names what the run DID
// record rather than only the number that was wanted.
func eventTypes(events []runEvent) string {
	if len(events) == 0 {
		return "no event"
	}

	types := make([]string, 0, len(events))

	for _, event := range events {
		types = append(types, event.Type)
	}

	return strings.Join(types, ", ")
}
