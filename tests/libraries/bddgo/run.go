package bddgo

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// ErrScenarioNotInRegistry signals a generated test naming a scenario
// the registry does not hold — a scenario deleted without regenerating.
var ErrScenarioNotInRegistry = errors.New("no scenario in the registry has this id")

// ErrScenarioNotOwned signals a generated test compiled into a suite
// whose service the scenario does not name. It would otherwise run
// against a state built for a different surface entirely.
var ErrScenarioNotOwned = errors.New("this suite does not own that scenario")

// ErrModifierWithoutBlock signals And/But before any Given/When/Then. A
// modifier inherits the block above it; with nothing above, there is
// nothing to inherit and no way to tell whether the step acts or asserts.
var ErrModifierWithoutBlock = errors.New("And/But before any Given/When/Then")

// ErrScenarioDrift signals a generated test file whose steps are not the
// registry's.
//
// The file is what runs — that is what makes it a test and not a
// comment — so a file the registry no longer agrees with is two specs
// where the project promises one. Refused whole rather than run
// partially: a scenario that executes the file's steps while its report
// quotes the registry's description is worse than one that does not run.
var ErrScenarioDrift = errors.New("the registry and this generated test file disagree")

// ErrDoneTwice signals Done called more than once on one scenario.
var ErrDoneTwice = errors.New("Done was already called")

// regenerate names the command that reconciles a drifted file. Quoted
// into every drift failure, because the fix is never a hand edit.
const regenerate = "true-bdd build tests --fix"

// Run is one scenario as its generated test file drives it.
//
// The file declares the steps by calling Given/When/Then/And/But, and
// Done runs them. Two things follow from that order, and both are the
// point. The steps are declared before anything executes, so the file
// reads as the scenario reads. And they are declared by the FILE, so
// what a reader sees in the test is what the machine does — the registry
// supplies the id, the description and the budget, and is checked
// against the file rather than substituted for it.
type Run[S any] struct {
	t     *testing.T
	suite *Suite[S]
	// registry is the scenario as the registry states it, used for the
	// cross-check and for everything that is not a step.
	registry Scenario
	// block is the last Given/When/Then keyword seen, which And/But
	// inherit. Empty until the first block keyword.
	block string
	// declared is what the FILE said, in call order.
	declared []declaredStep
	done     bool
}

// declaredStep is one Given/When/Then/And/But call, before classifying.
type declaredStep struct {
	keyword string
	// block is the Given/When/Then this step sits under, which is what
	// decides whether an `llm:`/`judge:` prefix is legal here. A `But`
	// under then: is a rule; the But is the reader's punctuation.
	block string
	// text is exactly what the file passed, prefix included.
	text string
}

// Scenario returns a Run for the named registry scenario.
//
// Refuses an id the registry does not hold, and one this suite does not
// own — the two ways a generated file can survive an edit to the
// registry that should have deleted or moved it.
func (s *Suite[S]) Scenario(t *testing.T, scenarioID string) *Run[S] {
	t.Helper()

	if s.err != nil {
		t.Fatalf("%v", s.err)
	}

	scenario, found, err := s.Lookup(scenarioID)
	if err != nil {
		t.Fatalf("bddgo: %v", err)
	}

	if !found {
		t.Fatalf("%s: %v (%s)\n  regenerate with: %s",
			scenarioID, ErrScenarioNotInRegistry, s.opts.Registry, regenerate)
	}

	if !s.spec.Owns(scenario) {
		t.Fatalf("%s: %v: scenario names service %q, suite %q covers %q\n  regenerate with: %s",
			scenarioID, ErrScenarioNotOwned, scenario.Service, s.spec.Name, s.spec.Service, regenerate)
	}

	run := &Run[S]{t: t, suite: s, registry: scenario}

	// A generated file that lost its Done() would otherwise be a test
	// that declares a scenario, runs nothing, and passes — the exact
	// shape of green this whole arrangement exists to make impossible.
	t.Cleanup(func() {
		if !run.done {
			t.Errorf("%s: Done was never called — the test declared %d step(s) and ran none\n  regenerate with: %s",
				scenarioID, len(run.declared), regenerate)
		}
	})

	return run
}

// Given opens the scenario's precondition block.
func (r *Run[S]) Given(text string) { r.open(KeywordGiven, text) }

// When opens the scenario's action block.
func (r *Run[S]) When(text string) { r.open(KeywordWhen, text) }

// Then opens the scenario's assertion block.
func (r *Run[S]) Then(text string) { r.open(KeywordThen, text) }

// And continues the block above it.
func (r *Run[S]) And(text string) { r.modify(KeywordAnd, text) }

// But continues the block above it. Interchangeable with And, exactly as
// Cucumber defines it; the convention here is to use it for a group's
// negative clause.
func (r *Run[S]) But(text string) { r.modify(KeywordBut, text) }

func (r *Run[S]) open(keyword, text string) {
	r.t.Helper()

	r.block = keyword
	r.declared = append(r.declared, declaredStep{keyword: keyword, block: keyword, text: text})
}

func (r *Run[S]) modify(keyword, text string) {
	r.t.Helper()

	if r.block == "" {
		r.t.Fatalf("%s: %v: %s %s\n  regenerate with: %s",
			r.registry.ID, ErrModifierWithoutBlock, keyword, text, regenerate)
	}

	r.declared = append(r.declared, declaredStep{keyword: keyword, block: r.block, text: text})
}

// Done cross-checks the declared steps against the registry and runs
// them. It is the last line of every generated test.
func (r *Run[S]) Done() {
	r.t.Helper()

	if r.done {
		r.t.Fatalf("%s: %v", r.registry.ID, ErrDoneTwice)
	}

	r.done = true

	steps, err := r.classify()
	if err != nil {
		r.t.Fatalf("%s: %v\n  regenerate with: %s", r.registry.ID, err, regenerate)
	}

	err = r.checkAgainstRegistry(steps)
	if err != nil {
		r.t.Fatalf("%v", err)
	}

	// The file supplies the behaviour; the registry supplies everything
	// that is not behaviour. A timeout is a fact about the machine a run
	// happens on rather than about what the run proves, so it belongs in
	// the document a person edits and not in generated code.
	scenario := r.registry
	scenario.Steps = steps

	r.suite.runScenario(r.t, scenario)
}

// classify resolves each declared step's mode against the block it sits
// in, using the same function the registry loader uses.
func (r *Run[S]) classify() ([]Step, error) {
	steps := make([]Step, 0, len(r.declared))

	for _, declared := range r.declared {
		text := strings.TrimSpace(declared.text)

		mode, text, err := Classify(text, declared.block)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", declared.text, err)
		}

		if mode != ModeDeterministic && text == "" {
			return nil, fmt.Errorf("%q: %w", declared.text, ErrEmptyClause)
		}

		steps = append(steps, Step{Keyword: declared.keyword, Mode: mode, Text: text})
	}

	return steps, nil
}

// checkAgainstRegistry compares the file's steps with the registry's,
// element by element, and reports the first that differs.
//
// The first rather than all of them: a step inserted near the top shifts
// every one below it, and a report listing forty differences would bury
// the single edit that caused them.
func (r *Run[S]) checkAgainstRegistry(steps []Step) error {
	want := r.registry.Steps

	for index := range max(len(steps), len(want)) {
		got, hasGot := stepAt(steps, index)
		expected, hasWant := stepAt(want, index)

		if hasGot && hasWant && got == expected {
			continue
		}

		return fmt.Errorf("%s: %w\n  step %d: file     %s\n           registry %s\n  regenerate with: %s",
			r.registry.ID, ErrScenarioDrift, index+1,
			describeStep(got, hasGot), describeStep(expected, hasWant), regenerate)
	}

	return nil
}

func stepAt(steps []Step, index int) (Step, bool) {
	if index >= len(steps) {
		return Step{}, false
	}

	return steps[index], true
}

// describeStep renders one side of a drift report, including the side
// that ran out of steps — "(no step)" is the whole diagnosis when a file
// is one step short.
func describeStep(step Step, present bool) string {
	if !present {
		return "(no step)"
	}

	return step.String()
}
