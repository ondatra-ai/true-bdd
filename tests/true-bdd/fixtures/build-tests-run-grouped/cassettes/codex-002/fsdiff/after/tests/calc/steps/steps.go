package steps

import (
	"fmt"
	"strconv"
)

func Register(suite Registrar) {
	suite.Step(`^the number is (-?\d+)$`, number)
	suite.Step(`^the number is doubled$`, double)
	suite.Step(`^the result is (-?\d+)$`, result)
}

// number seeds the harness with the number the scenario names.
func number(state *State, args []string) error {
	value, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("the number is %q: expected an integer, got %v", args[0], err)
	}
	state.Value = value
	return nil
}

// double is the action under test: it doubles the number in play.
func double(state *State, _ []string) error {
	state.Value *= 2
	return nil
}

// result asserts the number in play is the number the scenario expects.
func result(state *State, args []string) error {
	want, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("the result is %q: expected an integer, got %v", args[0], err)
	}
	if state.Value != want {
		return fmt.Errorf("expected the result to be %d, got %d", want, state.Value)
	}
	return nil
}
