package steps

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "running"
func conceptd811179f54fc202f6e4c73986e6f62affb7dc33a44ca0b952e967cf7a999da3d(state *State, args []string) error {
	command, number := args[0], args[1]
	if state.Program == "" {
		return errors.New("the calculator is not installed yet")
	}
	run := exec.Command(state.Program, command, number)
	run.Dir = "../.."
	output, err := run.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("run %q on %s: %w: %s", command, number, err, exit.Stderr)
		}
		return fmt.Errorf("run %q on %s: %w", command, number, err)
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return fmt.Errorf("read the integer stdout of %q on %s: %w", command, number, err)
	}
	state.Value = value
	return nil
}
