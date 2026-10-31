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
	// The declared CLI runs from the host root; the suite's tests run in
	// tests/calc, the same base the registry path "../../docs/scenarios.yaml"
	// is written against.
	const hostRoot = "../.."
	const packagePath = "./services/calc"

	number := args[0]
	if _, err := strconv.Atoi(number); err != nil {
		return fmt.Errorf("the Owner halves %q: not a whole number: %w", number, err)
	}

	cmd := exec.Command("go", "run", packagePath, "halve", number)
	cmd.Dir = hostRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go run %s halve %s: %w: %s", packagePath, number, err, strings.TrimSpace(stderr.String()))
	}

	result, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	if err != nil {
		return fmt.Errorf("go run %s halve %s: stdout %q is not an integer: %w", packagePath, number, stdout.String(), err)
	}

	state.Result = result
	return nil
}
