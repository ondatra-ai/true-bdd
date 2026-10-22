package steps

import ("fmt"; "strconv")

func Register(suite Registrar) {
 suite.Step(`^the number is (\d+)$`, number)
 suite.Step(`^the number is (doubled|tripled|halved|negated|squared)$`, transform)
 suite.Step(`^the result is (\d+)$`, result)
}

// transform applies the named arithmetic mutation to the number under
// test. The capture group carries the operation, so every scenario
// phrasing "the number is <op>" resolves to this one definition.
func transform(state *State, args []string) error {
 switch args[0] {
 case "doubled":
  state.Value *= 2
 case "tripled":
  state.Value *= 3
 case "halved":
  if state.Value%2 != 0 { return fmt.Errorf("expected an even number to halve, got %d", state.Value) }
  state.Value /= 2
 case "negated":
  state.Value = -state.Value
 case "squared":
  state.Value *= state.Value
 default:
  return fmt.Errorf("expected one of doubled, tripled, halved, negated, squared; got %q", args[0])
 }
 return nil
}
func number(state *State, args []string) error {
 value, err := strconv.Atoi(args[0]); state.Value = value; return err
}
func result(state *State, args []string) error {
 want, err := strconv.Atoi(args[0]); if err != nil { return err }
 if state.Value != want { return fmt.Errorf("expected %d, got %d", want, state.Value) }; return nil
}
