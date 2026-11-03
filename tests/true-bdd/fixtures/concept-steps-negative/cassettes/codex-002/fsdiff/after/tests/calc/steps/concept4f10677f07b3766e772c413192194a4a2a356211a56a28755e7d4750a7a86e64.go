package steps

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "halving"
func concept4f10677f07b3766e772c413192194a4a2a356211a56a28755e7d4750a7a86e64(state *State, args []string) error {
	const hostRoot = "../.."

	number, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("halving: step number %q is not a whole number: %w", args[0], err)
	}

	cmd := exec.Command("go", "run", "./services/calc", "halve", strconv.Itoa(number))
	cmd.Dir = hostRoot

	stdout, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf(
				"halving: halve %d exited with %d: %s",
				number,
				exitErr.ExitCode(),
				strings.TrimSpace(string(exitErr.Stderr)),
			)
		}
		return fmt.Errorf("halving: running halve %d failed: %w", number, err)
	}

	halved, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil {
		return fmt.Errorf(
			"halving: halve %d wrote %q on stdout, which is not an integer: %w",
			number,
			string(stdout),
			err,
		)
	}

	state.Result = halved
	return nil
}
