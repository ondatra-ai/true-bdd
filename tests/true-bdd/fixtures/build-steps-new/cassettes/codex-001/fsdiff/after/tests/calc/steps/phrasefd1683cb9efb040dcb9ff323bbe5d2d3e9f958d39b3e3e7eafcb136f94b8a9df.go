package steps

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func phrasefd1683cb9efb040dcb9ff323bbe5d2d3e9f958d39b3e3e7eafcb136f94b8a9df(state *State, args []string) error {
	const hostRoot = "../.."

	value := strconv.Itoa(state.Value)
	command := exec.Command("go", "run", "./services/calc", "double", value)
	command.Dir = hostRoot

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		return fmt.Errorf("double %s: %w: %s", value, err, strings.TrimSpace(stderr.String()))
	}

	doubled, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	if err != nil {
		return fmt.Errorf("double %s: stdout %q: %w", value, stdout.String(), err)
	}

	state.Value = doubled
	return nil
}
