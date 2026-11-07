package steps

import ("fmt"; "strconv")

func Register(suite Registrar) {
 suite.Step(`^the Owner doubles (-?\d+)$`, doubling)
 suite.Step(`^the result is (-?\d+)$`, result)
}

//truebdd:concept then "result"
func result(state *State, args []string) error {
 want, err := strconv.Atoi(args[0]); if err != nil { return err }
 if state.Value != want { return fmt.Errorf("expected %d, got %d", want, state.Value) }; return nil
}

//truebdd:concept when "doubling"
func doubling(state *State, args []string) error { value, err := strconv.Atoi(args[0]); state.Value = value * 2; return err }
