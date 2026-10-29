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
	const servicePackage = "./services/calc"

	argument := strconv.Itoa(state.Value)

	command := exec.Command("go", "run", servicePackage, "double", argument)
	command.Dir = hostRoot

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			return fmt.Errorf("double %s: %w", argument, err)
		}
		return fmt.Errorf("double %s: %w: %s", argument, err, detail)
	}

	doubled, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	if err != nil {
		return fmt.Errorf("double %s: unexpected stdout %q: %w", argument, stdout.String(), err)
	}

	state.Value = doubled
	return nil
}
