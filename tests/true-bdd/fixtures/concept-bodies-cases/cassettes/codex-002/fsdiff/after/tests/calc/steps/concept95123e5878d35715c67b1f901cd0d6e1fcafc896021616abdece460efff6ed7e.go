package steps

import (
	"fmt"
	"regexp"
)

//truebdd:concept then "output"
func concept95123e5878d35715c67b1f901cd0d6e1fcafc896021616abdece460efff6ed7e(state *State, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("then output: expected 2 captures (comparison, pattern), got %d", len(args))
	}
	comparison, pattern := args[0], args[1]

	// The when "operation" step keeps the command's stdout in state.Output with
	// its trailing newline already stripped; this step only observes it.
	output := state.Output

	expression, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("then output: the step's pattern %q is not a valid regular expression: %w", pattern, err)
	}
	matched := expression.MatchString(output)

	switch comparison {
	case "matches":
		if !matched {
			return fmt.Errorf("then output: the calculator's output %q does not match %q", output, pattern)
		}
	case "does not match":
		if matched {
			return fmt.Errorf("then output: the calculator's output %q matches %q, but the step requires no match", output, pattern)
		}
	default:
		return fmt.Errorf("then output: the step says %q, which is neither matches nor does not match", comparison)
	}
	return nil
}
