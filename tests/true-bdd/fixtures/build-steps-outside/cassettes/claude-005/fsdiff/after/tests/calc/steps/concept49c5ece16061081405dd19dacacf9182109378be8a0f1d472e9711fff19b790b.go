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
	command := exec.Command(state.Program, "double", value)
	command.Dir = "../.."
	stdout, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("run calc double %s: %w: %s", value, err, exit.Stderr)
		}
		return fmt.Errorf("run calc double %s: %w", value, err)
	}
	doubled, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil {
		return fmt.Errorf("read the doubled integer of %s from stdout %q: %w", value, stdout, err)
	}
	state.Value = doubled
	return nil
}
