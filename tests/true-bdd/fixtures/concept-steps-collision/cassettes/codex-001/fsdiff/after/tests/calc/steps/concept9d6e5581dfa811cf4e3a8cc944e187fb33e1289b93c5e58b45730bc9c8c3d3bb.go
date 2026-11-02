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
	number := args[0]

	cmd := exec.Command("go", "run", "./services/calc", "halve", number)
	cmd.Dir = "../.."

	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return fmt.Errorf("halve %s: %w: %s", number, err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return fmt.Errorf("halve %s: %w", number, err)
	}

	stdout := strings.TrimSpace(string(out))
	halved, err := strconv.Atoi(stdout)
	if err != nil {
		return fmt.Errorf("halve %s: stdout %q is not an integer: %w", number, stdout, err)
	}

	state.Result = halved
	return nil
}
