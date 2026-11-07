package steps

import "fixture/conceptbodiescases/tests/calc/helpers"

//truebdd:concept then "output"
func concept95123e5878d35715c67b1f901cd0d6e1fcafc896021616abdece460efff6ed7e(state *State, args []string) error {
	// What the step asked, read off its captures: the regular expression
	// taken as the rest of the step, and whether the step's wording demands
	// a match of it or demands that there be none.
	pattern, wantMatch, err := helpers.OutputExpectationFromStep(args)
	if err != nil {
		return err
	}

	// What the calculator printed for the operation, kept by the when step,
	// without the trailing newline its declaration prints after the value.
	printed, err := helpers.PrintedOutput(state.Output)
	if err != nil {
		return err
	}

	// Observe and compare only: the calculator already decided what to
	// print, and this step neither halves nor doubles anything to check it.
	return helpers.CheckOutputMatch(printed, pattern, wantMatch)
}

