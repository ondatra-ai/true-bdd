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
	if state.Program == "" {
		return errors.New("the calculator program is not installed yet")
	}
	command := exec.Command(state.Program, args[0], args[1])
	command.Dir = "../.."
	stdout, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("run %q on %s: %w: %s", args[0], args[1], err, exit.Stderr)
		}
		return fmt.Errorf("run %q on %s: %w", args[0], args[1], err)
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil {
		return fmt.Errorf("read the integer stdout of %q on %s: %w: %q", args[0], args[1], err, stdout)
	}
	state.Value = value
	return nil
}
