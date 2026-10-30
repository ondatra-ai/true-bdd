package steps

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "halving"
func concept8c83e3012dd37395db8aad8a8fafef5f14ca67ae936908b0e63bff4b60526148(state *State, args []string) error {
	const (
		hostRoot    = "../.."
		packagePath = "./services/calc"
		commandName = "halve"
	)
	if len(args) < 1 {
		return fmt.Errorf("%s: expected the whole number capture, got %d captures", commandName, len(args))
	}
	value, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("%s: the whole number %q is not an integer: %w", commandName, args[0], err)
	}
	cmd := exec.Command("go", "run", packagePath, commandName, strconv.Itoa(value))
	cmd.Dir = hostRoot
	var stderr strings.Builder
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("go run %s %s %d in %s: %w: %s", packagePath, commandName, value, hostRoot, err, strings.TrimSpace(stderr.String()))
	}
	halved, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil {
		return fmt.Errorf("%s: stdout %q is not an integer: %w", commandName, strings.TrimSpace(string(stdout)), err)
	}
	state.Result = halved
	return nil
}
