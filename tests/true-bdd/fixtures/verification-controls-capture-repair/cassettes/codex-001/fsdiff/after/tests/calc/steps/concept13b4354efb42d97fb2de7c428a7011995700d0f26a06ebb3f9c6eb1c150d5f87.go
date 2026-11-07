package steps

import (
	"fixture/conceptbodiescapture/tests/calc/helpers"
)

//truebdd:concept when "halving"
func concept13b4354efb42d97fb2de7c428a7011995700d0f26a06ebb3f9c6eb1c150d5f87(state *State, args []string) error {
	// args[0] is the whole number the Owner halves, captured in full by
	// `(-?\d+)`: an optional minus sign and every one of its digits, so "6"
	// arrives as "6", "64" as "64" and "-7" as "-7", never truncated to a
	// first digit.
	number := args[0]

	// Pass that number, unchanged, as the `value` argument of the calculator's
	// declared halve command, run as a process, and keep the integer it printed
	// on stdout as the result. The halving is the calculator's own work; this
	// step only carries its answer to the Then step.
	result, err := helpers.Halve(number)
	if err != nil {
		return err
	}
	state.Result = result
	return nil
}

