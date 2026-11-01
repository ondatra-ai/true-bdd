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
	// docs/architecture/calc-cli.yaml, command `halve`: run
	// `go run ./services/calc halve VALUE` from the host root; stdout is VALUE
	// halved and a newline. The suite's tests run from tests/calc — the same
	// directory the registry is read from as ../../docs/scenarios.yaml — so the
	// host root is two levels up.
	const hostRoot = "../.."

	value, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("halving: %q is not a whole number: %w", args[0], err)
	}

	cmd := exec.Command("go", "run", "./services/calc", "halve", strconv.Itoa(value))
	cmd.Dir = hostRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("halve %d: %w: %s", value, err, strings.TrimSpace(stderr.String()))
	}

	printed := strings.TrimSpace(stdout.String())
	halved, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("halve %d: stdout %q is not an integer: %w", value, printed, err)
	}

	state.Result = halved
	return nil
}
