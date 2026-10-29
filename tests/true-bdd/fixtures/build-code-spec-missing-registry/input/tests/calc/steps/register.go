package steps

import ("fmt"; "os/exec"; "strconv"; "strings")

func Register(suite Registrar) {
 suite.Step(`^the number is (\d+)$`, number)
 suite.Step(`^the number is doubled$`, double)
 suite.Step(`^the result is (\d+)$`, result)
}
func number(state *State, args []string) error { value, err := strconv.Atoi(args[0]); state.Value = value; return err }
func double(state *State, _ []string) error {
 command := exec.Command("go", "run", "./services/calc", "double", strconv.Itoa(state.Value))
 command.Dir = "../.."
 output, err := command.CombinedOutput(); if err != nil { return fmt.Errorf("calculator double: %w: %s", err, output) }
 state.Value, err = strconv.Atoi(strings.TrimSpace(string(output)))
 return err
}
func result(state *State, args []string) error {
 want, err := strconv.Atoi(args[0]); if err != nil { return err }
 if state.Value != want { return fmt.Errorf("expected %d, got %d", want, state.Value) }; return nil
}
