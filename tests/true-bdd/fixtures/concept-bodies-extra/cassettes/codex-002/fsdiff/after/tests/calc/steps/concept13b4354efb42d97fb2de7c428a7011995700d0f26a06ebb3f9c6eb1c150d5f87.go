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
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return fmt.Errorf("halve %s: %w: %s", number, err, message)
		}
		return fmt.Errorf("halve %s: %w", number, err)
	}
	result, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return fmt.Errorf("halve %s: stdout %q is not an integer", number, string(out))
	}
	state.Result = result
	return nil
}
