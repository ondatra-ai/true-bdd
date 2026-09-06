package registry

import (
	"errors"
	"fmt"
	"strings"
)

// Prefixes marking a step no regexp can settle. Mirrors bddgo's own
// vocabulary rather than importing it: pkg/testkit reaches the roots,
// never the other way round (ADR 0008).
const (
	// PrefixAct marks a step a model PERFORMS. Legal in given:/when: only.
	PrefixAct = "llm:"

	// PrefixRule marks a clause a model RULES ON. Legal in then: only.
	PrefixRule = "judge:"
)

// StepMode says who executes a step.
type StepMode int

const (
	// ModeDeterministic is a step bound to a step definition by regexp,
	// and the only mode step coverage can ask anything about.
	ModeDeterministic StepMode = iota
	// ModeAct is an `llm:` step: a model performs it, in place.
	ModeAct
	// ModeRule is a `judge:` clause, ruled on once after the run.
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

// classify peels a model-run prefix off a step's text and returns the
// mode it declares. blockKeyword is the BLOCK's keyword, never the
// step's own And/But. Byte-for-byte the rule bddgo.Classify applies.
func classify(text, blockKeyword string) (StepMode, string, error) {
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

// statementMode is classify plus bddgo flattenSteps' empty-clause check.
// Only the mode is returned: Statement.Text keeps its prefix verbatim,
// because the generated test quotes Text and bddgo strips it there.
func statementMode(text, blockKeyword string) (StepMode, error) {
	mode, stripped, err := classify(strings.TrimSpace(text), blockKeyword)
	if err != nil {
		return 0, fmt.Errorf("%q: %w", text, err)
	}

	if mode != ModeDeterministic && stripped == "" {
		return 0, fmt.Errorf("%q: %w", text, ErrEmptyClause)
	}

	return mode, nil
}
