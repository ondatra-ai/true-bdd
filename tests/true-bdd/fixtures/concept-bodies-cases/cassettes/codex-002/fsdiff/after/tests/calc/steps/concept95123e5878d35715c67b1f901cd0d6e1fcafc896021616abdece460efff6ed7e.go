package steps

import (
	"fmt"
	"regexp"
	"strings"
)

//truebdd:concept then "output"
func concept95123e5878d35715c67b1f901cd0d6e1fcafc896021616abdece460efff6ed7e(state *State, args []string) error {
	comparison, pattern := args[0], args[1]

	// The When step kept the command's stdout; drop a trailing newline here too so
	// the pattern is matched against the printed value alone, as the binding says.
	output := strings.TrimRight(state.Output, "\r\n")

	expression, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("the step's pattern %q is no Go regular expression: %w", pattern, err)
	}

	matched := expression.MatchString(output)

	switch comparison {
	case "matches":
		if !matched {
			return fmt.Errorf("the calculator's output %q has no match for %q", output, pattern)
		}
	case "does not match":
		if matched {
			return fmt.Errorf("the calculator's output %q has a match for %q", output, pattern)
		}
	default:
		return fmt.Errorf("the step names no declared comparison: %q", comparison)
	}

	return nil
}
