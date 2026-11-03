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
	const (
		command  = "double"
		hostRoot = "../.."
		notKept  = "the calculator program is not kept by the suite state; the installation step must run first"
	)
	if state.Program == "" {
		return errors.New(notKept)
	}
	value := args[0]
	if _, err := strconv.Atoi(value); err != nil {
		return fmt.Errorf("the whole number %q written in the step is not an integer: %w", value, err)
	}
	process := exec.Command(state.Program, command, value)
	process.Dir = hostRoot
	stdout, err := process.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("%s %s %s exited with %d: %s", state.Program, command, value, exit.ExitCode(), strings.TrimSpace(string(exit.Stderr)))
		}
		return fmt.Errorf("run %s %s %s: %w", state.Program, command, value, err)
	}
	printed := strings.TrimSpace(string(stdout))
	doubled, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("%s %s %s printed %q, which is not an integer: %w", state.Program, command, value, printed, err)
	}
	state.Value = doubled
	return nil
}
