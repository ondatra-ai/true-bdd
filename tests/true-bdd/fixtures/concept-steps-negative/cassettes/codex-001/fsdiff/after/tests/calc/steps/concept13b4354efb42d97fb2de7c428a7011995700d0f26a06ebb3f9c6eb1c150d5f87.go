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
	const hostRoot = "../.."
	const pkg = "./services/calc"
	const command = "halve"

	value := args[0]
	if _, err := strconv.Atoi(value); err != nil {
		return fmt.Errorf("halve value %q is not a whole number: %w", value, err)
	}

	cmd := exec.Command("go", "run", pkg, command, value)
	cmd.Dir = hostRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go run %s %s %s failed: %w: %s", pkg, command, value, err, strings.TrimSpace(stderr.String()))
	}

	out := strings.TrimSpace(stdout.String())
	halved, err := strconv.Atoi(out)
	if err != nil {
		return fmt.Errorf("go run %s %s %s printed %q, want an integer: %w", pkg, command, value, out, err)
	}

	state.Result = halved
	return nil
}
