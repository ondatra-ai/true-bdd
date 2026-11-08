package steps

import (
	"fmt"
	"strconv"

	"fixture/conceptbodiescapture/tests/calc/helpers"
)

//truebdd:concept when "halving"
func concept8c83e3012dd37395db8aad8a8fafef5f14ca67ae936908b0e63bff4b60526148(state *State, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("halving: expected the step's whole number as the only capture, got %d: %q", len(args), args)
	}
	number := args[0]
	if _, err := strconv.Atoi(number); err != nil {
		return fmt.Errorf("halving: captured %q is not a whole number: %w", number, err)
	}

	result, err := helpers.RunCalcInt("halve", number)
	if err != nil {
		return err
	}
	state.Result = result
	return nil
}

