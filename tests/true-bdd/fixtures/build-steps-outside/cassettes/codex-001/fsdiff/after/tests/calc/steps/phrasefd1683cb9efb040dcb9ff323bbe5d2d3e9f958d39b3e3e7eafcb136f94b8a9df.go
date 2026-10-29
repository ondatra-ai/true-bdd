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
		packagePath = "./services/calc"
		commandName = "double"
	)

	command := exec.Command("go", "run", packagePath, commandName, strconv.Itoa(state.Value))
	command.Dir = hostRoot

	stdout, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("run %s %s %d: %w: %s", packagePath, commandName, state.Value, err, strings.TrimSpace(string(exit.Stderr)))
		}
		return fmt.Errorf("run %s %s %d: %w", packagePath, commandName, state.Value, err)
	}

	doubled, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil {
		return fmt.Errorf("parse %s stdout %q: %w", commandName, string(stdout), err)
	}

	state.Value = doubled
	return nil
}
