package steps

import (
	"fmt"
	"strconv"

	"fixture/conceptbodiescapture/tests/calc/helpers"
)

//truebdd:concept when "halving"
func concept13b4354efb42d97fb2de7c428a7011995700d0f26a06ebb3f9c6eb1c150d5f87(state *State, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("the Owner halves: step needs the number capture, got %d captures", len(args))
	}
	// args[0] is the step's whole number, minus sign and every digit included.
	number, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("the Owner halves: %q is not a whole number: %w", args[0], err)
	}
	// The declared `halve` command runs as a process and alone decides the
	// integer; the step only carries the number in and the printed result out.
	halved, err := helpers.Halve(number)
	if err != nil {
		return err
	}
	state.Result = halved
	return nil
}
