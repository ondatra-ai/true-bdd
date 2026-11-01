package steps

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "halving"
func conceptec8b881314b929394828fe6c21c7db729a95bc1a93ad8abf8ec578b93a9e83f2(state *State, args []string) error {
	const (
		hostRoot    = "../.."
		packagePath = "./services/calc"
		command     = "halve"
	)
	number := args[0]
	cmd := exec.Command("go", "run", packagePath, command, number)
	cmd.Dir = hostRoot
	stdout, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("%s %s: %w: %s", command, number, err, strings.TrimSpace(string(exit.Stderr)))
		}
		return fmt.Errorf("%s %s: %w", command, number, err)
	}
	halved, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil {
		return fmt.Errorf("%s %s: stdout %q is not an integer: %w", command, number, string(stdout), err)
	}
	state.Result = halved
	return nil
}
