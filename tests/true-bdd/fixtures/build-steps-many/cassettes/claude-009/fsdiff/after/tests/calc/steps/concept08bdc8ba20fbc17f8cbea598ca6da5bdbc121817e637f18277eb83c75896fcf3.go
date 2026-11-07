package steps

import "fixture/stepsrun/tests/calc/helpers"

//truebdd:concept given "installation"
func concept08bdc8ba20fbc17f8cbea598ca6da5bdbc121817e637f18277eb83c75896fcf3(state *State, args []string) error {
	// The step has no captures: the command to build and where it is built
	// are fixed by the binding and the calc CLI declaration, so args is not
	// read. Build the declared calculator program under the host's tmp
	// directory and keep its path for the When.
	program, err := helpers.BuildCalculator()
	if err != nil {
		return err
	}
	state.CalculatorPath = program
	return nil
}
