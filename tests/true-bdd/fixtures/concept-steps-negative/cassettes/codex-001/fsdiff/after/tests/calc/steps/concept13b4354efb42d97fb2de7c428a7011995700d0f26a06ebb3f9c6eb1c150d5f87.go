package steps

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "halving"
func concept13b4354efb42d97fb2de7c428a7011995700d0f26a06ebb3f9c6eb1c150d5f87(state *State, args []string) error {
	const (
		hostRoot = "../.."
		pkg      = "./services/calc"
		command  = "halve"
	)
	number := args[0]
	cmd := exec.Command("go", "run", pkg, command, number)
	cmd.Dir = hostRoot
	stdout, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("go run %s %s %s: %w: %s", pkg, command, number, err, strings.TrimSpace(string(exit.Stderr)))
		}
		return fmt.Errorf("go run %s %s %s: %w", pkg, command, number, err)
	}
	printed := strings.TrimSpace(string(stdout))
	halved, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("%s %s printed %q, not an integer: %w", command, number, printed, err)
	}
	state.Result = halved
	return nil
}
