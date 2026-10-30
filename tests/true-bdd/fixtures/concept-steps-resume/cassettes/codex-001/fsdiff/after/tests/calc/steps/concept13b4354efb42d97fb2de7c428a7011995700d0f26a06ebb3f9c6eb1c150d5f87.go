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
	const hostRoot = "../.."

	number, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("step number %q is not a whole number: %w", args[0], err)
	}

	cmd := exec.Command("go", "run", "./services/calc", "halve", strconv.Itoa(number))
	cmd.Dir = hostRoot

	stdout, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf(
				"halve %d exited with %d: %s",
				number,
				exit.ExitCode(),
				strings.TrimSpace(string(exit.Stderr)),
			)
		}
		return fmt.Errorf("running halve %d: %w", number, err)
	}

	printed := strings.TrimSpace(string(stdout))

	result, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("halve %d printed %q on stdout, want an integer: %w", number, printed, err)
	}

	state.Result = result

	return nil
}
