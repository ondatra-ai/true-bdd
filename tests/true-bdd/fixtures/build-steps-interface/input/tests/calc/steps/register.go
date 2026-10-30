package steps

import ("fmt"; "os"; "os/exec"; "path/filepath"; "strconv"; "strings")

func Register(suite Registrar) {
 suite.Step(`^the calculator is installed$`, installed)
 suite.Step(`^the Owner doubles (-?\d+)$`, doubles)
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
func doubles(state *State, args []string) error {
 command := exec.Command(state.Program, "double", args[0])
 command.Dir = "../.."
 output, err := command.Output(); if err != nil { return fmt.Errorf("double %s: %w", args[0], err) }
 state.Value, err = strconv.Atoi(strings.TrimSpace(string(output)))
 return err
}
