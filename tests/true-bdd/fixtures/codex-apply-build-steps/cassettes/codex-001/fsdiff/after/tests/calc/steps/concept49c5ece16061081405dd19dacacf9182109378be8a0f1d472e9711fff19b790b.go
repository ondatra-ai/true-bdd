package steps

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "doubling"
func concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b(state *State, args []string) error {
	if state.Program == "" {
		return errors.New("the calculator is not installed")
	}
	command := exec.Command(state.Program, "double", args[0])
	command.Dir = "../.."
	stdout, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("run %s double %s: %w: %s", state.Program, args[0], err, exit.Stderr)
		}
		return fmt.Errorf("run %s double %s: %w", state.Program, args[0], err)
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil {
		return fmt.Errorf("read the double stdout %q: %w", stdout, err)
	}
	state.Value = value
	return nil
}
