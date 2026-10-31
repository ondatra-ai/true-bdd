package steps

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "doubling"
func concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b(state *State, args []string) error {
	if state.Program == "" {
		return fmt.Errorf("the calculator is not installed")
	}
	command := exec.Command(state.Program, "double", args[0])
	command.Dir = "../.."
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("run the calculator double %s: %w", args[0], err)
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return fmt.Errorf("read the doubled integer from %q: %w", string(output), err)
	}
	state.Value = value
	return nil
}
