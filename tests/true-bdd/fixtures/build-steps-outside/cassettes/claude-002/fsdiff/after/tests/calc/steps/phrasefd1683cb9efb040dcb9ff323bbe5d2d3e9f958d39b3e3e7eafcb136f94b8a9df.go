package steps

import (
	"bytes"
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

	argument := strconv.Itoa(state.Value)
	command := exec.Command("go", "run", packagePath, commandName, argument)
	command.Dir = hostRoot

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		return fmt.Errorf("run %s %s %s: %w: %s", packagePath, commandName, argument, err, strings.TrimSpace(stderr.String()))
	}

	doubled, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	if err != nil {
		return fmt.Errorf("parse %s stdout %q: %w", commandName, stdout.String(), err)
	}

	state.Value = doubled
	return nil
}
