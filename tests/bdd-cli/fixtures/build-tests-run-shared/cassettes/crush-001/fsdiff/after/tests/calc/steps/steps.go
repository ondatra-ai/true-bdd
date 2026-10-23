package steps

import ("fmt"; "strconv")

func Register(suite Registrar) {
 suite.Step(`^the number is (\d+)$`, number)
 suite.Step(`^the number is (doubled|tripled|halved)$`, scale)
 suite.Step(`^the result is (\d+)$`, result)
}
func number(state *State, args []string) error {
 value, err := strconv.Atoi(args[0]); state.Value = value; return err
}
func result(state *State, args []string) error {
 want, err := strconv.Atoi(args[0]); if err != nil { return err }
 if state.Value != want { return fmt.Errorf("expected %d, got %d", want, state.Value) }; return nil
}
func scale(state *State, args []string) error {
 switch args[0] {
 case "doubled":
  state.Value *= 2
 case "tripled":
  state.Value *= 3
 case "halved":
  if state.Value%2 != 0 { return fmt.Errorf("cannot halve %d: expected an even number", state.Value) }
  state.Value /= 2
 default:
  return fmt.Errorf("unknown operation %q: expected one of doubled, tripled, halved", args[0])
 }
 return nil
}
