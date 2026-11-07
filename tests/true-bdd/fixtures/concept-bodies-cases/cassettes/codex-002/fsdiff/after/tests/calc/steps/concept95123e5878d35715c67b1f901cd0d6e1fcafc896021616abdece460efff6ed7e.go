package steps

import (
	"fmt"

	"fixture/conceptbodiescases/tests/calc/helpers"
)

//truebdd:concept then "output"
func concept95123e5878d35715c67b1f901cd0d6e1fcafc896021616abdece460efff6ed7e(state *State, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("want 2 captures (comparison, pattern), got %d: %q", len(args), args)
	}
	comparison, pattern := args[0], args[1]

	// state.Output is the stdout the operation step kept, already without the
	// single trailing line terminator the CLI document declares. The CLI prints
	// a value on every successful run, so an empty kept output means no run was
	// observed rather than an empty answer, and the step says so instead of
	// silently passing a "does not match".
	if state.Output == "" {
		return fmt.Errorf(
			"no calculator output was kept, so it cannot be observed to %s %q: the operation step must run first",
			comparison, pattern,
		)
	}

	return helpers.CompareOutput(state.Output, comparison, pattern)
}
