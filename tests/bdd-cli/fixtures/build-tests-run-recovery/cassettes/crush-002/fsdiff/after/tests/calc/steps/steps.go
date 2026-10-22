package steps

import ("fmt"; "strconv")

func Register(suite Registrar) {
 suite.Step(`^the number is (\d+)$`, number)
 suite.Step(`^the number is doubled$`, func(s *State, _ []string) error { s.Value *= 2; return nil })
 suite.Step(`^the result is (\d+)$`, result)
}
func number(state *State, args []string) error {
 value, err := strconv.Atoi(args[0]); if err != nil { return err }
 state.Value = value; return nil
}
func result(state *State, args []string) error {
 want, err := strconv.Atoi(args[0]); if err != nil { return err }
 if state.Value != want { return fmt.Errorf("expected %d, got %d", want, state.Value) }; return nil
}
