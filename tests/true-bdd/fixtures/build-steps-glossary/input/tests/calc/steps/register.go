package steps

import ("fmt"; "os"; "os/exec"; "path/filepath"; "strconv")

func Register(suite Registrar) {
 suite.Step(`^the calculator is installed$`, installed)
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

//truebdd:concept then "result"
func result(state *State, args []string) error {
 want, err := strconv.Atoi(args[0]); if err != nil { return err }
 if state.Value != want { return fmt.Errorf("expected %d, got %d", want, state.Value) }; return nil
}
