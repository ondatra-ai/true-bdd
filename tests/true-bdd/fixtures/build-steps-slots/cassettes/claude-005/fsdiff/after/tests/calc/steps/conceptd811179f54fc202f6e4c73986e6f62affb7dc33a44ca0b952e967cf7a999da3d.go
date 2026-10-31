package steps

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "running"
func conceptd811179f54fc202f6e4c73986e6f62affb7dc33a44ca0b952e967cf7a999da3d(state *State, args []string) error {
	const hostRoot = "../.."
	if state.Program == "" {
		return fmt.Errorf("the calculator is not installed")
	}
	calculatorCommand := args[0]
	number := args[1]
	if _, err := strconv.Atoi(number); err != nil {
		return fmt.Errorf("the step's number %q is not a whole number: %w", number, err)
	}
	process := exec.Command(state.Program, calculatorCommand, number)
	process.Dir = hostRoot
	output, err := process.Output()
	if err != nil {
		return fmt.Errorf("run %q on %s: %w", calculatorCommand, number, err)
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return fmt.Errorf("read the integer stdout of %q on %s: %q: %w", calculatorCommand, number, output, err)
	}
	state.Value = value
	return nil
}
