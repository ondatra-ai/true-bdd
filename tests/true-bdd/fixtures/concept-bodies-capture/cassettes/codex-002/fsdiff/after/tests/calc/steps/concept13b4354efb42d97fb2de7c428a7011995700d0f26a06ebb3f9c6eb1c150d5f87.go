package steps

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "halving"
func concept13b4354efb42d97fb2de7c428a7011995700d0f26a06ebb3f9c6eb1c150d5f87(state *State, args []string) error {
	const (
		hostRoot    = "../.."
		servicePath = "./services/calc"
		commandName = "halve"
	)
	number := args[0]
	if _, err := strconv.Atoi(number); err != nil {
		return fmt.Errorf("halves step: %q is not a whole number: %w", number, err)
	}
	cmd := exec.Command("go", "run", servicePath, commandName, number)
	cmd.Dir = hostRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go run %s %s %s in %s: %w: %s", servicePath, commandName, number, hostRoot, err, strings.TrimSpace(stderr.String()))
	}
	result, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	if err != nil {
		return fmt.Errorf("%s %s stdout %q is not an integer: %w", commandName, number, stdout.String(), err)
	}
	state.Result = result
	return nil
}
