package steps

import (
	"fmt"

	"fixture/stepsrun/tests/calc/helpers"
)

//truebdd:concept when "doubling"
func concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b(state *State, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("the doubling step expects the whole number as its one value, got %d", len(args))
	}
	// args[0] is the whole number the step wrote, optionally signed. It is
	// handed to the calculator's double command verbatim as its VALUE
	// argument; the step performs no arithmetic of its own.
	value := args[0]
	doubled, err := helpers.RunDouble(state.Calculator, value)
	if err != nil {
		return err
	}
	// Keep the integer the program printed on stdout for a Then to observe.
	state.Doubled = doubled
	return nil
}
