package helpers

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// ComparisonMatches and ComparisonDoesNotMatch are the two alternatives a
	// step states to say whether the printed output must have a match for the
	// step's pattern or must have none.
	ComparisonMatches      = "matches"
	ComparisonDoesNotMatch = "does not match"
)

// PrintedOutput returns the kept stdout as the text a pattern is matched
// against: the calculator's CLI document says each command prints its result
// and a newline, so the one trailing newline is dropped and everything else is
// left verbatim.
func PrintedOutput(output string) string {
	return strings.TrimSuffix(output, "\n")
}

// MatchesPattern reports whether the step's pattern, read as a Go regular
// expression, has a match in printed. A pattern Go cannot compile is an error
// against the step rather than a silent non-match.
func MatchesPattern(printed, pattern string) (bool, error) {
	expression, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Errorf("the step's pattern %s is not a usable regular expression: %w", pattern, err)
	}

	return expression.MatchString(printed), nil
}
