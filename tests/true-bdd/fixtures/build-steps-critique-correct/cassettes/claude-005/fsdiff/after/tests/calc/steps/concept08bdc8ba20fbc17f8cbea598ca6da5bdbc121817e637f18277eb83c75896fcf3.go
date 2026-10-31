package steps

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

//truebdd:concept given "installation"
func concept08bdc8ba20fbc17f8cbea598ca6da5bdbc121817e637f18277eb83c75896fcf3(state *State, args []string) error {
	program, err := filepath.Abs("../../tmp/calc-program/calc")
	if err != nil {
		return fmt.Errorf("resolve calculator program path: %w", err)
	}

	build := exec.Command("go", "build", "-o", program, "./services/calc")
	build.Dir = "../.."
	output, err := build.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build calculator: %w: %s", err, output)
	}

	state.Program = program
	return nil
}
