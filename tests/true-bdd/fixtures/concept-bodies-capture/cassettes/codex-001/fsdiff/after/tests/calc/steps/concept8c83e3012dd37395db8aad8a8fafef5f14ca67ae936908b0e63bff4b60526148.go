package steps

import "fixture/conceptbodiescapture/tests/calc/helpers"

//truebdd:concept when "halving"
func concept8c83e3012dd37395db8aad8a8fafef5f14ca67ae936908b0e63bff4b60526148(state *State, args []string) error {
	// The Owner halves the step's whole number with the calculator:
	// run the declared `halve` command as a process with that number as
	// its argument, and keep the integer it printed on stdout as the
	// result for the observing step to compare.
	result, err := helpers.RunHalve(args[0])
	if err != nil {
		return err
	}
	state.Result = result
	return nil
}
