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
		return errors.New("double: the calculator is not installed")
	}
	command := exec.Command(state.Program, "double", args[0])
	command.Dir = "../.."
	output, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("double %s: %w: %s", args[0], err, exit.Stderr)
		}
		return fmt.Errorf("double %s: %w", args[0], err)
	}
	printed := strings.TrimSpace(string(output))
	value, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("double %s: unexpected stdout %q: %w", args[0], printed, err)
	}
	state.Value = value
	return nil
}
