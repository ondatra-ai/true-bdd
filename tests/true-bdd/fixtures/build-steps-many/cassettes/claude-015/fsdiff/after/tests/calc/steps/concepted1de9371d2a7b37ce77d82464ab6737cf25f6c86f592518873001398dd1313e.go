package steps

import (
	"fmt"
	"strconv"
)

//truebdd:concept then "result"
func concepted1de9371d2a7b37ce77d82464ab6737cf25f6c86f592518873001398dd1313e(state *State, args []string) error {
	expectedText := args[0]

	expected, err := strconv.Atoi(expectedText)
	if err != nil {
		return fmt.Errorf("the step's expected whole number %q is not an integer literal: %w", expectedText, err)
	}

	if !state.DoubledSet {
		return fmt.Errorf("no stdout of the calculator's double command was kept by the doubling When; the result step cannot observe the printed whole number it expects to be %d", expected)
	}

	if state.Doubled != expected {
		return fmt.Errorf("the calculator's double command printed %d on stdout, want %d", state.Doubled, expected)
	}

	return nil
}
