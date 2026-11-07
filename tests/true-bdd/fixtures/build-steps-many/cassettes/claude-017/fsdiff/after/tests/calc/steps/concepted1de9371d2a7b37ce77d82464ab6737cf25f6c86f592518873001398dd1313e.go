package steps

import (
	"fmt"
	"strconv"
)

//truebdd:concept then "result"
func concepted1de9371d2a7b37ce77d82464ab6737cf25f6c86f592518873001398dd1313e(state *State, args []string) error {
	// The pattern hands this body one capture: the whole number the step
	// expects the calculator to have printed, exactly as the step wrote it.
	if len(args) != 1 {
		return fmt.Errorf(
			"the result is: expected the step's whole number as the only capture, got %d",
			len(args),
		)
	}
	expectedText := args[0]
	expected, err := strconv.Atoi(expectedText)
	if err != nil {
		return fmt.Errorf(
			"the result is %s: the step's expected value %q is not a whole number: %w",
			expectedText, expectedText, err,
		)
	}
	// Empty text means the When never ran.
	if state.DoubledStdout == "" {
		return fmt.Errorf(
			"the result is %s: no result was observed from the calculator's %s command; the doubling When must run first",
			expectedText, "double",
		)
	}
	// Observe and compare only the number the program itself printed.
	if state.Doubled != expected {
		return fmt.Errorf(
			"the result is %s: the calculator's double command printed %q, the whole number %d",
			expectedText, state.DoubledStdout, state.Doubled,
		)
	}
	return nil
}
