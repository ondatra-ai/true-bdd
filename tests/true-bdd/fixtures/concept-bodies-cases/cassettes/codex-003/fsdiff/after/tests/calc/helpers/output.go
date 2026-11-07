package helpers

import (
	"fmt"
	"regexp"
	"strings"
)

// MatchesOutcome and DoesNotMatchOutcome are the two outcomes the output
// step states literally: whether the step requires its pattern to match
// what the calculator printed, or requires no match.
const (
	MatchesOutcome      = "matches"
	DoesNotMatchOutcome = "does not match"
)

// PrintedOutput is the text the declared CLI printed: the kept stdout
// stripped of the trailing newline the declaration says each of halve and
// double ends its result with. It only trims the line ending; it never
// reads, parses or rewrites what the calculator computed.
func PrintedOutput(stdout string) string {
	return strings.TrimSuffix(strings.TrimSuffix(stdout, "\n"), "\r")
}

// MatchOutput matches the step's pattern, as a Go regular expression,
// against the kept stdout without its trailing newline, and reports whether
// the outcome the step states holds: a match when the step says "matches",
// no match when it says "does not match". The calculator alone produced the
// output; this helper only observes it and compares it against the step's
// pattern.
func MatchOutput(stdout, outcome, pattern string) error {
	if stdout == "" {
		return fmt.Errorf("the calculator kept no output to match %q against: the operation step must run first", pattern)
	}
	printed := PrintedOutput(stdout)
	expression, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("the step's pattern %q is not a valid Go regular expression: %w", pattern, err)
	}
	matched := expression.MatchString(printed)
	switch outcome {
	case MatchesOutcome:
		if !matched {
			return fmt.Errorf("the calculator's output %q does not match %q", printed, pattern)
		}
		return nil
	case DoesNotMatchOutcome:
		if matched {
			return fmt.Errorf("the calculator's output %q matches %q", printed, pattern)
		}
		return nil
	default:
		return fmt.Errorf("the output step states no such outcome %q: it is either %q or %q", outcome, MatchesOutcome, DoesNotMatchOutcome)
	}
}
