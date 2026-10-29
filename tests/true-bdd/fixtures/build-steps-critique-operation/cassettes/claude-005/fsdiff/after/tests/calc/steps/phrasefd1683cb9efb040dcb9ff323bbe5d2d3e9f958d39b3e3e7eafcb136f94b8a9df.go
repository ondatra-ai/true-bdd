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
		hostRoot = "../.."
		pkg      = "./services/calc"
		command  = "double"
	)

	value := strconv.Itoa(state.Value)
	cmd := exec.Command("go", "run", pkg, command, value)
	cmd.Dir = hostRoot

	stdout, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("%s %s %s: %w: %s", pkg, command, value, err, exit.Stderr)
		}
		return fmt.Errorf("%s %s %s: %w", pkg, command, value, err)
	}

	doubled, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil {
		return fmt.Errorf("%s %s %s stdout %q: %w", pkg, command, value, stdout, err)
	}

	state.Value = doubled
	return nil
}
