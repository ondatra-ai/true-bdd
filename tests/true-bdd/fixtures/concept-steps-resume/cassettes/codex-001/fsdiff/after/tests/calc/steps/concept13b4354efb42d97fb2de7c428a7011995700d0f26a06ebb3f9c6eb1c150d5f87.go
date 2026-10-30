package steps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

//truebdd:concept when "halving"
func concept13b4354efb42d97fb2de7c428a7011995700d0f26a06ebb3f9c6eb1c150d5f87(state *State, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("the Owner halves: expected 1 capture, got %d", len(args))
	}
	value := args[0]
	if _, err := strconv.Atoi(value); err != nil {
		return fmt.Errorf("the Owner halves %q: not a whole number: %w", value, err)
	}

	// The declared CLI runs `go run ./services/calc halve VALUE` from the
	// host root; the suite's working directory is the scenario package, so
	// walk up to the directory holding go.mod.
	hostRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("the Owner halves %s: working directory: %w", value, err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(hostRoot, "go.mod")); statErr == nil {
			break
		}
		parent := filepath.Dir(hostRoot)
		if parent == hostRoot {
			return fmt.Errorf("the Owner halves %s: host root with go.mod not found above the suite directory", value)
		}
		hostRoot = parent
	}

	cmd := exec.Command("go", "run", "./services/calc", "halve", value)
	cmd.Dir = hostRoot
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		return fmt.Errorf("the Owner halves %s: go run ./services/calc halve %s in %s: %w: %s",
			value, value, hostRoot, runErr, strings.TrimSpace(stderr.String()))
	}

	out := strings.TrimSpace(stdout.String())
	halved, err := strconv.Atoi(out)
	if err != nil {
		return fmt.Errorf("the Owner halves %s: stdout %q is not an integer: %w", value, out, err)
	}
	state.Result = halved
	return nil
}
