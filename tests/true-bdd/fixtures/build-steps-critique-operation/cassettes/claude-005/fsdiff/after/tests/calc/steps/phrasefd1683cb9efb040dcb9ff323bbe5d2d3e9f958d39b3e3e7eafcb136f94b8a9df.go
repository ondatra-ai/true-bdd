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
		hostRoot   = "../.."
		servicePkg = "./services/calc"
		command    = "double"
	)

	cmd := exec.Command("go", "run", servicePkg, command, strconv.Itoa(state.Value))
	cmd.Dir = hostRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s %s %d: %w: %s", servicePkg, command, state.Value, err, strings.TrimSpace(stderr.String()))
	}

	doubled, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	if err != nil {
		return fmt.Errorf("%s stdout %q is not an integer: %w", command, stdout.String(), err)
	}
	state.Value = doubled
	return nil
}
