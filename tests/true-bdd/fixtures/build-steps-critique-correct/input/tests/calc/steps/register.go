package steps

import ("fmt"; "os/exec"; "strconv"; "strings")

func Register(suite Registrar) {
 suite.Step(`^the Owner doubles (-?\d+)$`, doubling)
 suite.Step(`^the result is (-?\d+)$`, result)
}

//truebdd:concept when "doubling"
func doubling(state *State, args []string) error {
 if state.Program == "" { return fmt.Errorf("the calculator is not installed") }
 command := exec.Command(state.Program, "double", args[0])
 command.Dir = "../.."
 output, err := command.CombinedOutput()
 if err != nil { return fmt.Errorf("calculator double: %w: %s", err, output) }
 state.Value, err = strconv.Atoi(strings.TrimSpace(string(output)))
 return err
}

//truebdd:concept then "result"
func result(state *State, args []string) error {
 want, err := strconv.Atoi(args[0]); if err != nil { return err }
 if state.Value != want { return fmt.Errorf("expected %d, got %d", want, state.Value) }; return nil
}
