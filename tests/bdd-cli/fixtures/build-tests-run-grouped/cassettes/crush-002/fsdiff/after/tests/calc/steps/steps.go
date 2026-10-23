package steps

import (
	"fmt"
	"strconv"
)

func Register(suite Registrar) {
	suite.Step(`^the number is (\d+)$`, number)
	suite.Step(`^the number is doubled$`, double)
	suite.Step(`^the result is (\d+)$`, result)
}

// number prepares the subject value from the step's captured digits.
func number(state *State, args []string) error {
	value, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("expected the number to be an integer, got %q", args[0])
	}
	state.Value = value
	return nil
}

// double acts: it doubles the value prepared by the Given step.
func double(state *State, _ []string) error {
	state.Value *= 2
	return nil
}

// result asserts the value against the step's captured expectation.
func result(state *State, args []string) error {
	want, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("expected the result to be an integer, got %q", args[0])
	}
	if state.Value != want {
		return fmt.Errorf("expected %d, got %d", want, state.Value)
	}
	return nil
}
