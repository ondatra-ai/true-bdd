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
	// The declared CLI says the calculator program is run from the host root as
	// "calc double VALUE" and prints the doubled integer and a newline on
	// stdout; the binding says this When runs that command as a process with
	// the step's whole number and keeps its integer stdout.
	const (
		commandName = "double"
		stepLabel   = "the Owner doubles"
	)

	if len(args) != 1 {
		return fmt.Errorf("%s: expected 1 capture (the whole number), got %d", stepLabel, len(args))
	}
	value := args[0]

	if state.Program == "" {
		return fmt.Errorf("%s %s: no calculator program kept; the installation Given must run first", stepLabel, value)
	}
	if state.HostRoot == "" {
		return fmt.Errorf("%s %s: no host root kept; the installation Given must run first", stepLabel, value)
	}

	state.Result = 0
	state.ResultSet = false

	run := exec.Command(state.Program, commandName, value)
	run.Dir = state.HostRoot
	stdout, err := run.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("%s %s: %s %s %s in %s: %w: %s", stepLabel, value, state.Program, commandName, value, state.HostRoot, err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return fmt.Errorf("%s %s: %s %s %s in %s: %w", stepLabel, value, state.Program, commandName, value, state.HostRoot, err)
	}

	printed := strings.TrimSpace(string(stdout))
	doubled, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("%s %s: stdout %q of %s %s %s is not an integer: %w", stepLabel, value, printed, state.Program, commandName, value, err)
	}

	state.Result = doubled
	state.ResultSet = true

	return nil
}
