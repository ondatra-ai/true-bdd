// This file holds the output observation the calc suite's Then steps share.
//
// Nothing here produces or predicts what the calculator prints: the printed
// text arrives as the suite kept it, and the step's pattern arrives as the step
// wrote it. The only work done is compiling that pattern and reporting whether
// it has a match in the kept text.
package helpers

import (
	"fmt"
	"regexp"
)

// The words the comparison capture can hold, and what each one requires of the
// kept output.
const (
	// ComparisonMatches requires the step's pattern to have a match in the
	// kept stdout.
	ComparisonMatches = "matches"

	// ComparisonDoesNotMatch requires the step's pattern to have no match in
	// the kept stdout.
	ComparisonDoesNotMatch = "does not match"
)

// CompareOutput matches pattern, a Go regular expression taken verbatim from
// the step, against output, the stdout the suite kept without its trailing
// newline, and reports whether the comparison the step named holds.
//
// The pattern is unanchored on purpose: it is used exactly as the step wrote
// it, so a step that wants the whole line to be the value anchors it itself.
func CompareOutput(output, comparison, pattern string) error {
	expr, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf(
			"the step's pattern %q is not a Go regular expression: %w",
			pattern, err,
		)
	}

	matched := expr.MatchString(output)

	switch comparison {
	case ComparisonMatches:
		if !matched {
			return fmt.Errorf(
				"the calculator printed %q, which has no match for %q",
				output, pattern,
			)
		}
		return nil
	case ComparisonDoesNotMatch:
		if matched {
			return fmt.Errorf(
				"the calculator printed %q, which has a match for %q",
				output, pattern,
			)
		}
		return nil
	default:
		return fmt.Errorf(
			"comparison capture %q is neither %q nor %q",
			comparison, ComparisonMatches, ComparisonDoesNotMatch,
		)
	}
}
