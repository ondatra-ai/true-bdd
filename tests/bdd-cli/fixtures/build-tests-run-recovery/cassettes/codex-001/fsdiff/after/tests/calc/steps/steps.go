package steps

import ("fmt"; "strconv")

func Register(suite Registrar) {
 suite.Step(`^the number is (\d+)$`, number)
 suite.Step(`^the result is (\d+)$`, result)
 suite.Step(`^the number is (doubled|tripled|halved|negated)$`, mutate)
}
func number(state *State, args []string) error {
 value, err := strconv.Atoi(args[0]); state.Value = value; return err
}
func result(state *State, args []string) error {
 want, err := strconv.Atoi(args[0]); if err != nil { return err }
 if state.Value != want { return fmt.Errorf("expected %d, got %d", want, state.Value) }; return nil
}

// mutate is the When-side vocabulary for the unary arithmetic the registry
// phrases as "the number is <verb>". It acts on the shared State.Value that
// `number` prepares and `result` asserts over.
func mutate(state *State, args []string) error {
 if len(args) != 1 {
  return fmt.Errorf("expected 1 mutation argument, got %d", len(args))
 }
 switch args[0] {
 case "doubled":
  state.Value *= 2
 case "tripled":
  state.Value *= 3
 case "negated":
  state.Value = -state.Value
 case "halved":
  if state.Value%2 != 0 {
   return fmt.Errorf("expected an even number to halve, got %d", state.Value)
  }
  state.Value /= 2
 default:
  return fmt.Errorf("expected one of doubled, tripled, halved, negated; got %q", args[0])
 }
 return nil
}
