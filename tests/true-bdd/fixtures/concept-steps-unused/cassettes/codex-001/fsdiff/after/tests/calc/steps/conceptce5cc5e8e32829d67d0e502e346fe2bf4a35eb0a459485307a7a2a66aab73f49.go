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
		return fmt.Errorf("whole number to halve %q: %w", args[0], err)
	}
	halved, err := helpers.RunInt("halve", strconv.Itoa(operand))
	if err != nil {
		return err
	}
	state.Result = halved
	return nil
}
