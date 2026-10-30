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
	number := args[0]
	cmd := exec.Command("go", "run", "./services/calc", "halve", number)
	cmd.Dir = hostRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return fmt.Errorf("halve %s: %w: %s", number, err, message)
		}
		return fmt.Errorf("halve %s: %w", number, err)
	}
	result, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	if err != nil {
		return fmt.Errorf("halve %s: stdout %q is not an integer: %w", number, stdout.String(), err)
	}
	state.Result = result
	return nil
}
