package steps

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "halving"
func concept9d6e5581dfa811cf4e3a8cc944e187fb33e1289b93c5e58b45730bc9c8c3d3bb(state *State, args []string) error {
	cmd := exec.Command("go", "run", "./services/calc", "halve", args[0])
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("halve %s: %w: %s", args[0], err, strings.TrimSpace(string(exit.Stderr)))
		}
		return fmt.Errorf("halve %s: %w", args[0], err)
	}
	result, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return fmt.Errorf("halve %s: stdout %q is not an integer: %w", args[0], string(out), err)
	}
	state.Result = result
	return nil
}
