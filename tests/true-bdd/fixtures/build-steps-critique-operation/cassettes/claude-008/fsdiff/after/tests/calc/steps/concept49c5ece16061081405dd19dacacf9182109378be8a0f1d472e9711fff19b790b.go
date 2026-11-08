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
	// The suite runs from tests/calc, so the host root the calculator is
	// invoked from is two levels up -- the same directory the given step
	// built the program in.
	const hostRoot = "../.."

	value, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("the whole number to double: %w", err)
	}
	if state.Program == "" {
		return errors.New("the calculator is not installed")
	}

	command := exec.Command(state.Program, "double", strconv.Itoa(value))
	command.Dir = hostRoot
	output, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("double %d: %w: %s", value, err, exit.Stderr)
		}
		return fmt.Errorf("double %d: %w", value, err)
	}

	doubled, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return fmt.Errorf("double %d printed %q: %w", value, output, err)
	}
	state.Value = doubled
	return nil
}
