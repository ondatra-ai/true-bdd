package steps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

//truebdd:concept given "installation"
func concept08bdc8ba20fbc17f8cbea598ca6da5bdbc121817e637f18277eb83c75896fcf3(state *State, args []string) error {
	if err := os.MkdirAll("../../tmp/calc-program", 0o755); err != nil {
		return fmt.Errorf("installation: create tmp dir: %w", err)
	}

	program, err := filepath.Abs("../../tmp/calc-program/calc")
	if err != nil {
		return fmt.Errorf("installation: resolve program path: %w", err)
	}

	command := exec.Command("go", "build", "-o", "tmp/calc-program/calc", "./services/calc")
	command.Dir = "../.."
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("installation: go build ./services/calc: %w: %s", err, output)
	}

	state.Program = program
	return nil
}
