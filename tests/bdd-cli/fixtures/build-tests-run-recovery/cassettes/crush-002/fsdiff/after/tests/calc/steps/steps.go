package steps

import ("fmt"; "strconv")

func Register(suite Registrar) {
	suite.Step(`^the number is (\d+)$`, number)
	suite.Step(`^the number is doubled$`, double)
	suite.Step(`^the result is (\d+)$`, result)
}

// number prepares the state with the number named by the step.
func number(state *State, args []string) error {
	value, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("expected a whole number, got %q: %w", args[0], err)
	}
	state.Value = value
	return nil
}

// double acts: it doubles the number currently held in state.
func double(state *State, _ []string) error {
	state.Value *= 2
	return nil
}
func result(state *State, args []string) error {
 want, err := strconv.Atoi(args[0]); if err != nil { return err }
 if state.Value != want { return fmt.Errorf("expected %d, got %d", want, state.Value) }; return nil
}
