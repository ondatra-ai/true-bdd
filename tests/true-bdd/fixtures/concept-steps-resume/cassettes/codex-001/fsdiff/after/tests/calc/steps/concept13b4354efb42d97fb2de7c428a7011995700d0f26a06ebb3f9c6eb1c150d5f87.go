package steps

import (
	"fmt"
	"strconv"

	"fixture/conceptsteps/tests/calc/helpers"
)

//truebdd:concept when "halving"
func concept13b4354efb42d97fb2de7c428a7011995700d0f26a06ebb3f9c6eb1c150d5f87(state *State, args []string) error {
	number, err := helpers.WholeNumber(args[0])
	if err != nil {
		return fmt.Errorf("the Owner halves %s: %w", args[0], err)
	}

	result, err := helpers.RunCalcInt(helpers.HalveCommand, strconv.Itoa(number))
	if err != nil {
		return err
	}

	state.Result = result
	return nil
}
