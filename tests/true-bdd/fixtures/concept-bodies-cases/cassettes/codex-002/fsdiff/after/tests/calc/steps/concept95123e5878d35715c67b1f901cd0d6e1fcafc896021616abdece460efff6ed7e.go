package steps

import (
	"fmt"
	"regexp"
	"strings"
)

//truebdd:concept then "output"
func concept95123e5878d35715c67b1f901cd0d6e1fcafc896021616abdece460efff6ed7e(state *State, args []string) error {
	comparison, pattern := args[0], args[1]

	if state.Output == "" {
		return fmt.Errorf("no output was kept, so the step cannot match %q: the operation step must run first", pattern)
	}

	// The declared commands print the result and a newline, and the binding
	// matches the pattern against the kept stdout without that trailing
	// newline.
	printed := strings.TrimSuffix(state.Output, "\n")

	expression, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("the step's pattern %q is no regular expression: %w", pattern, err)
	}
	matched := expression.MatchString(printed)

	switch comparison {
	case "matches":
		if !matched {
			return fmt.Errorf("the calculator's output %q does not match %q", printed, pattern)
		}
	case "does not match":
		if matched {
			return fmt.Errorf("the calculator's output %q matches %q", printed, pattern)
		}
	default:
		return fmt.Errorf("the step names no declared comparison: %q", comparison)
	}

	return nil
}
