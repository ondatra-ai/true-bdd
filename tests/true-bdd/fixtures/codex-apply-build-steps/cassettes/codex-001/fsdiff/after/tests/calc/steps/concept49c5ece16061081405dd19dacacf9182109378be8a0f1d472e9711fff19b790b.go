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
	value := args[0]
	if _, err := strconv.Atoi(value); err != nil {
		return fmt.Errorf("the whole number %q to double: %w", value, err)
	}
	command := exec.Command(state.Program, "double", value)
	command.Dir = "../.."
	stdout, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("double %s: %w: %s", value, err, exit.Stderr)
		}
		return fmt.Errorf("double %s: %w", value, err)
	}
	doubled, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil {
		return fmt.Errorf("double %s printed %q, want an integer: %w", value, stdout, err)
	}
	state.Value = doubled
	return nil
}
