package steps

import "fixture/stepsrun/tests/calc/helpers"

//truebdd:concept given "installation"
func concept08bdc8ba20fbc17f8cbea598ca6da5bdbc121817e637f18277eb83c75896fcf3(state *State, args []string) error {
	// Build the declared calculator command as a program under the host's
	// tmp directory and keep its path for the When. The step carries no
	// value, so there is nothing in args to read.
	calculator, err := helpers.BuildCalculator()
	if err != nil {
		return err
	}
	state.Calculator = calculator
	return nil
}
