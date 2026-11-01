package steps

import ("fmt"; "os"; "os/exec"; "path/filepath"; "strconv"; "strings")

func Register(suite Registrar) {
 suite.Step(`^the calculator is installed$`, installed)
 suite.Step(`^the Owner doubles (-?\d+)$`, doubling)
 suite.Step(`^the Owner halves (-?\d+)$`, halving)
 suite.Step(`^the result is (-?\d+)$`, result)
}

//truebdd:concept given "installation"
func installed(state *State, _ []string) error {
 program, err := filepath.Abs("../../tmp/calc-program/calc"); if err != nil { return err }
 if err = os.MkdirAll(filepath.Dir(program), 0o755); err != nil { return err }
 command := exec.Command("go", "build", "-o", program, "./services/calc")
 command.Dir = "../.."
 if output, err := command.CombinedOutput(); err != nil { return fmt.Errorf("install the calculator: %w: %s", err, output) }
 state.Program = program
 return nil
}

//truebdd:concept when "doubling"
func doubling(state *State, args []string) error {
 if state.Program == "" { return fmt.Errorf("the calculator is not installed") }
 output, err := exec.Command(state.Program, "double", args[0]).CombinedOutput()
 if err != nil { return fmt.Errorf("calculator double: %w: %s", err, output) }
 state.Value, err = strconv.Atoi(strings.TrimSpace(string(output)))
 return err
}

//truebdd:concept when "halving"
func halving(state *State, args []string) error {
 if state.Program == "" { return fmt.Errorf("the calculator is not installed") }
 output, err := exec.Command(state.Program, "halve", args[0]).CombinedOutput()
 if err != nil { return fmt.Errorf("calculator halve: %w: %s", err, output) }
 state.Value, err = strconv.Atoi(strings.TrimSpace(string(output)))
 return err
}

//truebdd:concept then "result"
func result(state *State, args []string) error {
 want, err := strconv.Atoi(args[0]); if err != nil { return err }
 if state.Value != want { return fmt.Errorf("expected %d, got %d", want, state.Value) }; return nil
}
