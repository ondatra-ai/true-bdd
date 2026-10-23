package steps

import ("fmt"; "strconv")

func Register(suite Registrar) {
 suite.Step(`^the number is (\d+)$`, number)
 suite.Step(`^the number is (doubled|tripled|quadrupled)$`, scaled)
 suite.Step(`^the result is (\d+)$`, result)
}

// multipliers maps the registry's scaling vocabulary to its factor, so one
// definition serves every scenario phrasing "the number is <operation>"
// instead of a pattern copied from a single scenario's line.
var multipliers = map[string]int{"doubled": 2, "tripled": 3, "quadrupled": 4}

func scaled(state *State, args []string) error {
 factor, ok := multipliers[args[0]]
 if !ok {
  return fmt.Errorf("expected one of doubled, tripled, quadrupled; got %q", args[0])
 }
 state.Value *= factor
 return nil
}
func number(state *State, args []string) error {
 value, err := strconv.Atoi(args[0]); state.Value = value; return err
}
func result(state *State, args []string) error {
 want, err := strconv.Atoi(args[0]); if err != nil { return err }
 if state.Value != want { return fmt.Errorf("expected %d, got %d", want, state.Value) }; return nil
}
