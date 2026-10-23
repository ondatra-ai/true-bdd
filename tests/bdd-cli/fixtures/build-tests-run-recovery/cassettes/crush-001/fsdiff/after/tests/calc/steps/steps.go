package steps

import ("fmt"; "strconv")

func Register(suite Registrar) {
 suite.Step(`^the number is (\d+)$`, number)
 suite.Step(`^the result is (\d+)$`, result)
 suite.Step(`^the number is (doubled|tripled|halved)$`, scaled)
}
func number(state *State, args []string) error {
 value, err := strconv.Atoi(args[0]); state.Value = value; return err
}
func result(state *State, args []string) error {
 want, err := strconv.Atoi(args[0]); if err != nil { return err }
 if state.Value != want { return fmt.Errorf("expected %d, got %d", want, state.Value) }; return nil
}
func scaled(state *State, args []string) error {
 switch args[0] {
 case "doubled": state.Value *= 2
 case "tripled": state.Value *= 3
 case "halved":
  if state.Value%2 != 0 { return fmt.Errorf("expected an even number to halve, got %d", state.Value) }
  state.Value /= 2
 default: return fmt.Errorf("unsupported operation %q on number %d", args[0], state.Value)
 }
 return nil
}
