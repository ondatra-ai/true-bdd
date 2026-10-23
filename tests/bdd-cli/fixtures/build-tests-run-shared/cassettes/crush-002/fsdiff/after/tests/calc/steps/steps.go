package steps

import ("fmt"; "strconv")

func Register(suite Registrar) {
 suite.Step(`^the number is (\d+)$`, number)
 suite.Step(`^the number is doubled$`, doubled)
 suite.Step(`^the result is (\d+)$`, result)
}

// number prepares the state: it stores the captured number and nothing else.
func number(state *State, args []string) error {
 value, err := strconv.Atoi(args[0])
 if err != nil {
  return fmt.Errorf("the number is %q: expected an integer, got an unparsable value: %w", args[0], err)
 }
 state.Value = value
 return nil
}

// doubled acts on the state: it multiplies the current number by two.
func doubled(state *State, _ []string) error {
 state.Value *= 2
 return nil
}
func result(state *State, args []string) error {
 want, err := strconv.Atoi(args[0]); if err != nil { return err }
 if state.Value != want { return fmt.Errorf("expected %d, got %d", want, state.Value) }; return nil
}
