package steps

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "halving"
func concept8c83e3012dd37395db8aad8a8fafef5f14ca67ae936908b0e63bff4b60526148(state *State, args []string) error {
	value := args[0]
	if _, err := strconv.Atoi(value); err != nil {
		return fmt.Errorf("the Owner halves %q: not a whole number: %w", value, err)
	}
	cmd := exec.Command("go", "run", "./services/calc", "halve", value)
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf(
				"halve %s exited with status %d: %s",
				value,
				exitErr.ExitCode(),
				strings.TrimSpace(string(exitErr.Stderr)),
			)
		}
		return fmt.Errorf("running halve %s: %w", value, err)
	}
	halved, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return fmt.Errorf("halve %s printed %q on stdout, not an integer: %w", value, string(out), err)
	}
	state.Result = halved
	return nil
}
