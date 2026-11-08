package steps

import (
	"fmt"

	"fixture/stepsrun/tests/calc/helpers"
)

//truebdd:concept when "doubling"
func concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b(state *State, args []string) error {
	// The pattern hands this body one capture: the whole number the Owner
	// doubles, exactly as the step wrote it. That text is the double
	// command's value argument and all the body knows of the step.
	if len(args) != 1 {
		return fmt.Errorf(
			"the Owner doubles: expected the step's whole number as the only capture, got %d",
			len(args),
		)
	}
	value := args[0]
	if state.CalculatorPath == "" {
		return fmt.Errorf(
			"the Owner doubles %s: no calculator program is installed; the installation Given must run first",
			value,
		)
	}
	// Run the kept program's double command as a process from the host root
	// and keep the integer stdout it prints. The body passes the step's text
	// straight through and never doubles the number itself.
	stdout, err := helpers.RunDouble(state.CalculatorPath, value)
	if err != nil {
		return err
	}
	doubled, err := helpers.DecodeInteger(stdout)
	if err != nil {
		return fmt.Errorf("the Owner doubles %s: %w", value, err)
	}
	state.DoubledStdout = stdout
	state.Doubled = doubled
	return nil
}
