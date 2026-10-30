package steps

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

//truebdd:concept when "doubling"
func concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b(state *State, args []string) error {
	const commandName = "double"

	number := args[0]

	if state.CalcBinary == "" {
		return fmt.Errorf(
			"run the %q command with %s: the calculator program is not installed yet",
			commandName, number,
		)
	}

	state.DoubleStdout = ""

	var stdout, stderr bytes.Buffer

	run := exec.Command(state.CalcBinary, commandName, number)
	run.Dir = state.HostRoot
	run.Stdout = &stdout
	run.Stderr = &stderr

	if err := run.Run(); err != nil {
		return fmt.Errorf(
			"run %s %s %s from the host root %s: %w: %s",
			state.CalcBinary, commandName, number, state.HostRoot, err, strings.TrimSpace(stderr.String()),
		)
	}

	state.DoubleStdout = stdout.String()

	return nil
}
