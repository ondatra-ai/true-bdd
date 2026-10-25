package steps

import ("fmt"; "strconv")

func Register(suite Registrar) {
 suite.Step(`^the number is (\d+)$`, number)
 suite.Step(`^the number is (doubled|tripled|quadrupled|halved)$`, scale)
 suite.Step(`^the result is (\d+)$`, result)
}
func number(state *State, args []string) error {
 value, err := strconv.Atoi(args[0]); state.Value = value; return err
}
func scale(state *State, args []string) error {
 factors := map[string]int{"doubled": 2, "tripled": 3, "quadrupled": 4}
 if args[0] == "halved" {
  if state.Value%2 != 0 { return fmt.Errorf("cannot halve %d: expected an even number", state.Value) }
  state.Value /= 2
  return nil
 }
 factor, ok := factors[args[0]]
 if !ok { return fmt.Errorf("unknown scaling %q: expected one of doubled, tripled, quadrupled, halved", args[0]) }
 state.Value *= factor
 return nil
}
func result(state *State, args []string) error {
 want, err := strconv.Atoi(args[0]); if err != nil { return err }
 if state.Value != want { return fmt.Errorf("expected %d, got %d", want, state.Value) }; return nil
}
