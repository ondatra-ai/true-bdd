package steps

import ("fmt"; "strconv")

func Register(suite Registrar) {
 suite.Step(`^the number is (\d+)$`, number)
 suite.Step(`^the number is (doubled|tripled|squared|halved)$`, applyOperation)
 suite.Step(`^the result is (\d+)$`, result)
}
func number(state *State, args []string) error {
 value, err := strconv.Atoi(args[0]); state.Value = value; return err
}
func result(state *State, args []string) error {
 want, err := strconv.Atoi(args[0]); if err != nil { return err }
 if state.Value != want { return fmt.Errorf("expected %d, got %d", want, state.Value) }; return nil
}

// applyOperation performs the unary arithmetic the When step names on the
// number already prepared in state. The operation word is captured, so one
// definition serves every scenario phrasing an operation this way rather
// than one definition per scenario line. The alternation is spelled out in
// words on purpose: a `(\w+)` capture here would also match
// "the number is 2" and leave that step bound to two definitions.
func applyOperation(state *State, args []string) error {
 switch args[0] {
 case "doubled":
  state.Value *= 2
 case "tripled":
  state.Value *= 3
 case "squared":
  state.Value *= state.Value
 case "halved":
  if state.Value%2 != 0 {
   return fmt.Errorf("expected an even number to halve, got %d", state.Value)
  }
  state.Value /= 2
 default:
  return fmt.Errorf("unsupported operation %q", args[0])
 }
 return nil
}
