package steps

import (
	"fmt"
	"strconv"

	"fixture/conceptbodiescapture/tests/calc/helpers"
)

//truebdd:concept when "halving"
func concept8c83e3012dd37395db8aad8a8fafef5f14ca67ae936908b0e63bff4b60526148(state *State, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("the Owner halves: step needs the number capture, got %d captures", len(args))
	}
	number, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("the Owner halves: %q is not a whole number: %w", args[0], err)
	}
	halved, err := helpers.Halve(number)
	if err != nil {
		return err
	}
	state.Result = halved
	return nil
}
