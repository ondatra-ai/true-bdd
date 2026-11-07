package steps

import (
	"fmt"

	"fixture/stepsrun/tests/calc/helpers"
)

//truebdd:concept then "result"
func concepted1de9371d2a7b37ce77d82464ab6737cf25f6c86f592518873001398dd1313e(state *State, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("the result Then expects 1 capture (the expected whole number), got %d", len(args))
	}

	expected, err := helpers.ExpectedNumber(args[0])
	if err != nil {
		return err
	}

	if !state.DoubledSet {
		return fmt.Errorf("no %s command output kept: the doubling When must run first", helpers.DoubleCommand)
	}

	if state.Doubled != expected {
		return fmt.Errorf("the calculator's %s command printed %d, want %d",
			helpers.DoubleCommand, state.Doubled, expected)
	}
	return nil
}
