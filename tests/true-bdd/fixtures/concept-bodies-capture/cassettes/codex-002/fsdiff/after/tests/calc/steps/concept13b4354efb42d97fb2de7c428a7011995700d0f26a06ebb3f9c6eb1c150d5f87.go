package steps

import "fixture/conceptbodiescapture/tests/calc/helpers"

//truebdd:concept when "halving"
func concept13b4354efb42d97fb2de7c428a7011995700d0f26a06ebb3f9c6eb1c150d5f87(state *State, args []string) error {
	// The Owner halves the step's whole number with the calculator.
	//
	// args[0] is the step's number in full, sign included ("6", "64",
	// "-7"): it is handed to the declared `halve` command unchanged as
	// that command's one value argument. The integer the command printed
	// on stdout is kept as the result for the observing step to compare;
	// this step computes no half of its own.
	result, err := helpers.RunHalve(args[0])
	if err != nil {
		return err
	}
	state.Result = result
	return nil
}
