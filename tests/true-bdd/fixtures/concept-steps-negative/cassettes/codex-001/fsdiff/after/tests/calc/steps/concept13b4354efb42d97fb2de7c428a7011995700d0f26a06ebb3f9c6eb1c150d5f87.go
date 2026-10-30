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
		packagePath = "./services/calc"
		commandName = "halve"
	)
	number := args[0]
	if _, err := strconv.Atoi(number); err != nil {
		return fmt.Errorf("%s: captured number %q is not a whole number: %w", commandName, number, err)
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("go", "run", packagePath, commandName, number)
	cmd.Dir = hostRoot
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w: %s", commandName, number, err, strings.TrimSpace(stderr.String()))
	}
	value, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	if err != nil {
		return fmt.Errorf("%s %s: stdout %q is not an integer: %w", commandName, number, stdout.String(), err)
	}
	state.Result = value
	return nil
}
