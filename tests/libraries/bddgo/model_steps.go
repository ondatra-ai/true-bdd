package bddgo

import (
	"errors"
	"fmt"
	"strings"
)

// Prefixes marking a step no regexp can settle. Each is legal in exactly
// one place, because the two roles are not interchangeable: `llm:` makes
// something happen, `judge:` rules on what already did.
const (
	// PrefixAct marks a step a model PERFORMS. Legal in given:/when: only,
	// for an operation stated by intent rather than mechanism — e.g.
	// "close whatever dialog is covering the list".
	PrefixAct = "llm:"

	// PrefixRule marks a clause a model RULES ON. Legal in then: only, for
	// an assertion about meaning (two wordings saying the same thing) —
	// anything a comparison can settle belongs in a step definition instead.
	PrefixRule = "judge:"
)

// StepMode says who executes a step.
type StepMode int

const (
	// ModeDeterministic is a step bound to a step definition by regexp.
	// The default, and what every step without a prefix is.
	ModeDeterministic StepMode = iota
	// ModeAct is an `llm:` step: a model performs it, in place.
	ModeAct
	// ModeRule is a `judge:` clause: it is collected while the scenario
	// runs and ruled on once, at the end.
	ModeRule
)

// ErrPrefixWrongBlock signals a model-run prefix in a block that cannot
// run it — refused rather than reinterpreted, since the two prefixes
// name different engines a silent swap would make unpredictable.
var ErrPrefixWrongBlock = errors.New("model-run step prefix is not legal in this block")

// ErrEmptyClause signals a prefix with nothing after it. The clause IS
// the whole instruction the model receives; an empty one asks it to act
// on nothing, and would pass or fail on the model's mood.
var ErrEmptyClause = errors.New("model-run step has no text after its prefix")

// ErrNoActor signals an `llm:` step against a suite whose state cannot
// act — e.g. a CLI scenario, whose state has no Actor implementation.
// The step stops the run rather than quietly doing nothing.
var ErrNoActor = errors.New("scenario has an llm: step but the suite's state does not implement bddgo.Actor")

// ErrNoJudge signals judge: clauses against a state that cannot grade
// them — refused rather than skipped, since an ungraded clause would
// still read as covered: the suite reports green, nothing checked it.
var ErrNoJudge = errors.New("scenario has judge: clauses but the suite's state does not implement bddgo.Judgeable")

// Actor is the optional interface a suite's state implements to host
// `llm:` steps. Act runs one such step at its position, so a later
// step sees whatever it did. Nothing implements it yet (forward work).
type Actor interface {
	Act(step Step) error
}

// Judgeable is the optional interface a suite's state implements to
// host `judge:` clauses. Judge is called once, with every clause of the
// scenario in registry order, only after every other step passed.
type Judgeable interface {
	Judge(clauses []Step) error
}

// Classify peels a model-run prefix off a step's text and returns the
// mode it declares; the prefix never reaches Text. blockKeyword is the
// BLOCK's keyword, never the step's own And/But.
func Classify(text, blockKeyword string) (StepMode, string, error) {
	switch {
	case strings.HasPrefix(text, PrefixAct):
		if blockKeyword == KeywordThen {
			return 0, "", fmt.Errorf("%w: %q acts, and then: asserts — use %q for a clause to be ruled on",
				ErrPrefixWrongBlock, PrefixAct, PrefixRule)
		}

		return ModeAct, strings.TrimSpace(strings.TrimPrefix(text, PrefixAct)), nil

	case strings.HasPrefix(text, PrefixRule):
		if blockKeyword != KeywordThen {
			return 0, "", fmt.Errorf("%w: %q is a verdict, and only then: follows the run — use %q to make something happen",
				ErrPrefixWrongBlock, PrefixRule, PrefixAct)
		}

		return ModeRule, strings.TrimSpace(strings.TrimPrefix(text, PrefixRule)), nil

	default:
		return ModeDeterministic, text, nil
	}
}
