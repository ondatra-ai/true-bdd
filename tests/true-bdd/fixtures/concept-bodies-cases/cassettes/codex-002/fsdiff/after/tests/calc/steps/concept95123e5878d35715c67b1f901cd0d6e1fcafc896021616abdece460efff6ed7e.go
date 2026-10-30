package steps

import (
	"fmt"
	"regexp"
)

//truebdd:concept then "output"
func concept95123e5878d35715c67b1f901cd0d6e1fcafc896021616abdece460efff6ed7e(state *State, args []string) error {
	comparison, pattern := args[0], args[1]

	const (
		matches      = "matches"
		doesNotMatch = "does not match"
	)

	expression, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("the step's pattern is no regular expression: %q: %w", pattern, err)
	}

	found := expression.MatchString(state.Output)

	switch comparison {
	case matches:
		if !found {
			return fmt.Errorf("the calculator printed %q, which the pattern %q has no match in", state.Output, pattern)
		}
	case doesNotMatch:
		if found {
			return fmt.Errorf("the calculator printed %q, which the pattern %q has a match in", state.Output, pattern)
		}
	default:
		return fmt.Errorf("the step names no declared comparison: %q", comparison)
	}

	return nil
}
