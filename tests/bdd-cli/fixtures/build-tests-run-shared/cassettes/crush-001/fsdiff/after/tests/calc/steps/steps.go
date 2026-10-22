package steps

import ("fmt"; "strconv")

func Register(suite Registrar) {
 suite.Step(`^the number is (\d+)$`, number)
 suite.Step(`^the result is (\d+)$`, result)
 suite.Step(`^the number is (doubled|tripled|squared|negated|halved)$`, transform)
}
func number(state *State, args []string) error {
 value, err := strconv.Atoi(args[0]); state.Value = value; return err
}
func result(state *State, args []string) error {
 want, err := strconv.Atoi(args[0]); if err != nil { return err }
 if state.Value != want { return fmt.Errorf("expected %d, got %d", want, state.Value) }; return nil
}
func transform(state *State, args []string) error {
 operation := args[0]; before := state.Value
 switch operation {
 case "doubled": state.Value = before * 2
 case "tripled": state.Value = before * 3
 case "squared": state.Value = before * before
 case "negated": state.Value = -before
 case "halved":
  if before%2 != 0 { return fmt.Errorf("expected an even number to halve, got %d", before) }
  state.Value = before / 2
 default:
  return fmt.Errorf("expected one of doubled, tripled, squared, negated, halved; got %q", operation)
 }
 return nil
}
