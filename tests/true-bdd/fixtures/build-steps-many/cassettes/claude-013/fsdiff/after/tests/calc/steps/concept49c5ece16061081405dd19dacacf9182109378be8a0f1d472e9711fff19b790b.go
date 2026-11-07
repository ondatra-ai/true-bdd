package steps

import (
	"fmt"

	"fixture/stepsrun/tests/calc/helpers"
)

//truebdd:concept when "doubling"
func concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b(state *State, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("the doubling When expects 1 capture (the whole number), got %d", len(args))
	}
	value := args[0]

	doubled, err := helpers.RunDouble(state.CalcPath, value)
	if err != nil {
		return err
	}

	state.Doubled = doubled
	state.DoubledSet = true
	return nil
}
