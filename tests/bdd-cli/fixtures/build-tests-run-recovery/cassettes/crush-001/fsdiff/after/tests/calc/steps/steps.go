package steps

import ("fmt"; "strconv")

func Register(suite Registrar) {
 suite.Step(`^the number is (\d+)$`, number)
 suite.Step(`^the result is (\d+)$`, result)
 suite.Step(`^the number is (doubled|tripled|quadrupled)$`, scaled)
}
func number(state *State, args []string) error {
 value, err := strconv.Atoi(args[0]); state.Value = value; return err
}
func result(state *State, args []string) error {
 want, err := strconv.Atoi(args[0]); if err != nil { return err }
 if state.Value != want { return fmt.Errorf("expected %d, got %d", want, state.Value) }; return nil
}
func scaled(state *State, args []string) error {
 factors := map[string]int{"doubled": 2, "tripled": 3, "quadrupled": 4}
 factor, ok := factors[args[0]]
 if !ok { return fmt.Errorf("expected one of doubled, tripled or quadrupled, got %q", args[0]) }
 state.Value *= factor; return nil
}
