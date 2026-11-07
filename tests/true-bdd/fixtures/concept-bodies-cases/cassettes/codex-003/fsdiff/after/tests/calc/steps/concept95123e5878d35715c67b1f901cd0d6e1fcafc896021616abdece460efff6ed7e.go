package steps

import (
	"fixture/conceptbodiescases/tests/calc/helpers"
)

//truebdd:concept then "output"
func concept95123e5878d35715c67b1f901cd0d6e1fcafc896021616abdece460efff6ed7e(state *State, args []string) error {
	outcome, pattern := args[0], args[1]

	// Observe only: the operation step already kept the declared command's
	// stdout. Match the step's pattern against that kept output without its
	// trailing newline and require the outcome the step states — a match for
	// "matches", no match for "does not match". The calculator alone worked
	// out what it printed; nothing here halves, doubles or otherwise
	// reproduces the result being checked.
	return helpers.MatchOutput(state.Output, outcome, pattern)
}
