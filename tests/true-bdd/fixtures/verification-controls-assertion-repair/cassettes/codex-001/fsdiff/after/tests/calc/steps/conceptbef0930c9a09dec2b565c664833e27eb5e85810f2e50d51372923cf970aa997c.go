package steps

import (
	"fmt"

	"fixture/stepsrun/tests/calc/helpers"
)

//truebdd:concept given "installation"
func conceptbef0930c9a09dec2b565c664833e27eb5e85810f2e50d51372923cf970aa997c(state *State, args []string) error {
	program, err := helpers.BuildCalculator()
	if err != nil {
		return fmt.Errorf("installation: %w", err)
	}
	state.Program = program
	return nil
}

