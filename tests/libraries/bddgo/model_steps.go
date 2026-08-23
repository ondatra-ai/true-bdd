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
	// PrefixAct marks a step a model PERFORMS. Legal in given:/when:.
	//
	// Its use is an operation stated by intent rather than by mechanism —
	// "close whatever dialog is covering the list" — where the actor has
	// to look at the page to work out what that means.
	PrefixAct = "llm:"

	// PrefixRule marks a clause a model RULES ON. Legal in then: only.
	//
	// Its use is an assertion about meaning: that two wordings say the
	// same thing, that a rewrite kept its substance. Everything a
	// comparison can settle belongs in a step definition instead.
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
// run it.
//
// Refused rather than reinterpreted. The two prefixes name two different
// engines — one spawns an agent turn mid-scenario, the other contributes
// a line to a verdict taken after every other step passed — and a step
// that silently swapped engines depending on which block it was pasted
// into would be a step whose behaviour no reader could predict from its
// text.
var ErrPrefixWrongBlock = errors.New("model-run step prefix is not legal in this block")

// ErrEmptyClause signals a prefix with nothing after it. The clause IS
// the whole instruction the model receives; an empty one asks it to act
// on nothing, and would pass or fail on the model's mood.
var ErrEmptyClause = errors.New("model-run step has no text after its prefix")

// ErrNoActor signals an `llm:` step against a suite whose state cannot
// act.
//
// This is what confines acting to the surfaces where it earns its keep.
// A CLI scenario's state has a subprocess and a working tree, nothing an
// agent could usefully look at, so it does not implement Actor and an
// `llm:` step in a CLI scenario stops the run instead of quietly doing
// nothing.
var ErrNoActor = errors.New("scenario has an llm: step but the suite's state does not implement bddgo.Actor")

// ErrNoJudge signals judge: clauses against a state that cannot grade
// them.
//
// Refused rather than skipped, and the reason is sharper than for an
// unbound step: an ungraded clause still READS as coverage. The registry
// would state the requirement, the suite would report green, and nothing
// would ever have checked it.
var ErrNoJudge = errors.New("scenario has judge: clauses but the suite's state does not implement bddgo.Judgeable")

// Actor is the optional interface a suite's state implements to host
// `llm:` steps. Act runs one such step at its position in the scenario,
// so a later step sees whatever it did.
//
// Nothing implements it yet, and that is deliberate forward work rather
// than an oversight. The bdd-web scenarios are where acting earns its
// keep: "close whatever dialog is covering the list" is a real step, and
// naming a testid in it would be a spoiler — the scenario would encode
// the answer it means to check. So `llm:` is a declared capability with
// no implementer until those step definitions land.
type Actor interface {
	Act(step Step) error
}

// Judgeable is the optional interface a suite's state implements to host
// `judge:` clauses.
//
// Judge is called ONCE, with every clause of the scenario in registry
// order, and only after all its other steps passed — bddgo aborts a
// scenario at its first failing step, so a run that never reached its
// assertions never reaches its verdict either. One call rather than one
// per clause because a verdict is taken on the run as a whole, and
// because the evidence a judge reads is the same for all of them.
type Judgeable interface {
	Judge(clauses []Step) error
}

// Classify peels a model-run prefix off a step's text and returns the
// mode it declares.
//
// The prefix never reaches Text. A step definition is bound by wording,
// and a judge reads its clause as a sentence; the prefix says who runs
// the step, which is the loader's business and nobody else's.
//
// blockKeyword is the keyword of the BLOCK the step sits in, never the
// And/But a step may have spelled for itself — `- but: 'judge: …'` in a
// then: block is a rule, and reads as one.
//
// Exported because a generated test file transcribes a step's raw text,
// prefix and all, and the run has to reach the same verdict about it as
// the registry loader did. Two classifiers would eventually disagree,
// and the disagreement would be a scenario that runs differently from
// the way it reads.
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
