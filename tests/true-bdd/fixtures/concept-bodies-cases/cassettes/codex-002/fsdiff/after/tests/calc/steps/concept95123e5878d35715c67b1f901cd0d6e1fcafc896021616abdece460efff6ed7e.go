package steps

import (
	"fmt"

	"fixture/conceptbodiescases/tests/calc/helpers"
)

//truebdd:concept then "output"
func concept95123e5878d35715c67b1f901cd0d6e1fcafc896021616abdece460efff6ed7e(state *State, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("expected 2 captures (comparison, pattern), got %d: %q", len(args), args)
	}
	comparison, pattern := args[0], args[1]

	if state.Output == "" {
		return fmt.Errorf("the calculator printed nothing to match %s against: no run of the calculator was kept", pattern)
	}

	printed := helpers.PrintedOutput(state.Output)

	matched, err := helpers.MatchesPattern(printed, pattern)
	if err != nil {
		return err
	}

	switch comparison {
	case helpers.ComparisonMatches:
		if !matched {
			return fmt.Errorf("the calculator printed %q, which has no match for %s", printed, pattern)
		}
	case helpers.ComparisonDoesNotMatch:
		if matched {
			return fmt.Errorf("the calculator printed %q, which has a match for %s, but the step requires none", printed, pattern)
		}
	default:
		return fmt.Errorf("unknown comparison %q: a step says either %q or %q", comparison, helpers.ComparisonMatches, helpers.ComparisonDoesNotMatch)
	}

	return nil
}
