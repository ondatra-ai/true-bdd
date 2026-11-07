package steps

import (
	"fmt"

	"fixture/conceptbodiescases/tests/calc/helpers"
)

//truebdd:concept when "operation"
func concept7b86a45f32a2af1379e8139ecafca2e1995ea6f5ea105d09465e201fb17f9fd4(state *State, args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("want 3 captures (exact, operation, number), got %d: %q", len(args), args)
	}
	exact, operation, number := args[0], args[1], args[2]

	argv, err := helpers.CalculatorArgs(operation, exact, number)
	if err != nil {
		return err
	}

	output, err := helpers.RunCalculator(argv...)
	if err != nil {
		return err
	}

	state.Output = output
	return nil
}
