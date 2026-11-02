package steps

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

//truebdd:concept when "halving"
func concept13b4354efb42d97fb2de7c428a7011995700d0f26a06ebb3f9c6eb1c150d5f87(state *State, args []string) error {
	value, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("the Owner halves %q: not a whole number: %w", args[0], err)
	}

	// The declared CLI document says: run `go run ./services/calc halve VALUE`
	// from the host root. `go test` runs this suite from tests/calc, so the
	// host root is two directories up.
	const (
		hostRoot    = "../.."
		servicePath = "./services/calc"
		command     = "halve"
		timeout     = 2 * time.Minute
	)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", servicePath, command, strconv.Itoa(value))
	cmd.Dir = hostRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s %d: %w: %s", servicePath, command, value, err, strings.TrimSpace(stderr.String()))
	}

	out := strings.TrimSpace(stdout.String())
	result, err := strconv.Atoi(out)
	if err != nil {
		return fmt.Errorf("%s %s %d: stdout %q is not an integer", servicePath, command, value, out)
	}

	state.Result = result
	return nil
}
