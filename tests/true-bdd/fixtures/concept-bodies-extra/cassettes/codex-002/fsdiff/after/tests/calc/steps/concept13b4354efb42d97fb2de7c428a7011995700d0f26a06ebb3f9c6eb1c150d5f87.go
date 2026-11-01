package steps

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "halving"
func concept13b4354efb42d97fb2de7c428a7011995700d0f26a06ebb3f9c6eb1c150d5f87(state *State, args []string) error {
	const hostRoot = "../.."
	number := args[0]
	if _, err := strconv.Atoi(number); err != nil {
		return fmt.Errorf("the step's number %q is not a whole number: %w", number, err)
	}
	cmd := exec.Command("go", "run", "./services/calc", "halve", number)
	cmd.Dir = hostRoot
	stdout, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("halve %s exited %d: %s", number, exit.ExitCode(), strings.TrimSpace(string(exit.Stderr)))
		}
		return fmt.Errorf("running halve %s: %w", number, err)
	}
	halved, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil {
		return fmt.Errorf("halve %s printed %q, not an integer: %w", number, strings.TrimSpace(string(stdout)), err)
	}
	state.Result = halved
	return nil
}
