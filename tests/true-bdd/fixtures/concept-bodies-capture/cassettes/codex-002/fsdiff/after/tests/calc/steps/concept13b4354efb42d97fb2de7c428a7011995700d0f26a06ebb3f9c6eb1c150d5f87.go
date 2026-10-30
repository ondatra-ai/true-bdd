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
		hostRoot    = "../.."
		calcPackage = "./services/calc"
	)

	number := args[0]

	cmd := exec.Command("go", "run", calcPackage, "halve", number)
	cmd.Dir = hostRoot

	stdout, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("halve %s: %w: %s", number, err, strings.TrimSpace(string(exit.Stderr)))
		}
		return fmt.Errorf("halve %s: %w", number, err)
	}

	halved, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil {
		return fmt.Errorf("halve %s: stdout %q is not an integer: %w", number, string(stdout), err)
	}

	state.Result = halved
	return nil
}
