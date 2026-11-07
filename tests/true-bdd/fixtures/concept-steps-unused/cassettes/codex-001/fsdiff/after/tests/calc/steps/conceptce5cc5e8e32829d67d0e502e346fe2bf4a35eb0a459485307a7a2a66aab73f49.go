package steps

import (
	"fmt"
	"strconv"

	"fixture/conceptsteps/tests/calc/helpers"
)

//truebdd:concept when "halving"
func conceptce5cc5e8e32829d67d0e502e346fe2bf4a35eb0a459485307a7a2a66aab73f49(state *State, args []string) error {
	operand, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("the Owner's number %q is not a whole number: %w", args[0], err)
	}

	halved, err := helpers.RunHalve(operand)
	if err != nil {
		return err
	}

	state.Result = halved
	return nil
}
