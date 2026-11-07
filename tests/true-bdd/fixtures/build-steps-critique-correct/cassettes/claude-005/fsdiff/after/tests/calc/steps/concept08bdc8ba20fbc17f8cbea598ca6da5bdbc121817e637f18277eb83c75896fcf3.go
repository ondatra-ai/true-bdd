package steps

import (
	"fmt"

	"fixture/stepsrun/tests/calc/helpers"
)

//truebdd:concept given "installation"
func concept08bdc8ba20fbc17f8cbea598ca6da5bdbc121817e637f18277eb83c75896fcf3(state *State, args []string) error {
	program, err := helpers.BuildCalculator()
	if err != nil {
		return fmt.Errorf("install calculator: %w", err)
	}
	state.Program = program
	return nil
}
