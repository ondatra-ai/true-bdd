package steps

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func phrasefd1683cb9efb040dcb9ff323bbe5d2d3e9f958d39b3e3e7eafcb136f94b8a9df(state *State, args []string) error {
	const (
		hostRoot    = "../.."
		servicePath = "./services/calc"
		commandName = "double"
	)

	value := strconv.Itoa(state.Value)
	cmd := exec.Command("go", "run", servicePath, commandName, value)
	cmd.Dir = hostRoot

	stdout, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("%s %s: %w: %s", commandName, value, err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return fmt.Errorf("%s %s: %w", commandName, value, err)
	}

	doubled, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil {
		return fmt.Errorf("%s %s stdout %q: %w", commandName, value, string(stdout), err)
	}

	state.Value = doubled
	return nil
}
