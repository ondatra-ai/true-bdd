package steps

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "halving"
func concept8c83e3012dd37395db8aad8a8fafef5f14ca67ae936908b0e63bff4b60526148(state *State, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("the Owner halves: expected 1 capture, got %d", len(args))
	}
	value := args[0]
	if _, err := strconv.Atoi(value); err != nil {
		return fmt.Errorf("the Owner halves %q: not a whole number: %w", value, err)
	}
	cmd := exec.Command("go", "run", "./services/calc", "halve", value)
	cmd.Dir = "../.."
	stdout, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("halve %s: %w: %s", value, err, strings.TrimSpace(string(exit.Stderr)))
		}
		return fmt.Errorf("halve %s: %w", value, err)
	}
	printed := strings.TrimSpace(string(stdout))
	halved, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("halve %s: stdout %q is not an integer: %w", value, printed, err)
	}
	state.Result = halved
	return nil
}
