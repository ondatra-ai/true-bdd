package steps

import "fixture/stepsrun/tests/calc/helpers"

//truebdd:concept when "doubling"
func concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b(state *State, args []string) error {
	doubled, err := helpers.Double(state.Program, args[0])
	if err != nil {
		return err
	}
	state.Value = doubled
	return nil
}
