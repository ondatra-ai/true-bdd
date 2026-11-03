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
		return errors.New("double a number: the calculator is not installed")
	}
	number := args[0]
	if _, err := strconv.Atoi(number); err != nil {
		return fmt.Errorf("double a number: the step's whole number %q: %w", number, err)
	}
	command := exec.Command(state.Program, "double", number)
	command.Dir = "../.."
	stdout, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("double %s: %w: %s", number, err, exit.Stderr)
		}
		return fmt.Errorf("double %s: %w", number, err)
	}
	printed := strings.TrimSpace(string(stdout))
	doubled, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("double %s: read the printed integer %q: %w", number, printed, err)
	}
	state.Value = doubled
	return nil
}
