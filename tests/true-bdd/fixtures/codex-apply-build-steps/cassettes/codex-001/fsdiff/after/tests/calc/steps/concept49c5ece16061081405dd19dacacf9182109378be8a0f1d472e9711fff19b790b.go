package steps

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "doubling"
func concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b(state *State, args []string) error {
	const hostRoot = "../.."
	if state.Program == "" {
		return fmt.Errorf("the calculator is not installed")
	}
	command := exec.Command(state.Program, "double", args[0])
	command.Dir = hostRoot
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("double %s: %w: %s", args[0], err, output)
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return fmt.Errorf("double %s: unexpected stdout %q: %w", args[0], string(output), err)
	}
	state.Value = value
	return nil
}
