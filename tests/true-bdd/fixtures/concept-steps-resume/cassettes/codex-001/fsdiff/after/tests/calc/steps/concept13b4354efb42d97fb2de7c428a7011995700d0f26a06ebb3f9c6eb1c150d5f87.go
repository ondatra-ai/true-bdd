package steps

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

//truebdd:concept when "halving"
func concept13b4354efb42d97fb2de7c428a7011995700d0f26a06ebb3f9c6eb1c150d5f87(state *State, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("halve: want 1 capture, got %d", len(args))
	}
	number, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("halve: whole number %q: %w", args[0], err)
	}

	// The suite runs with tests/calc as its working directory (see the
	// registry path the scenarios package passes to bddgo), and the halve
	// command is declared to run from the host root.
	hostRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return fmt.Errorf("halve %d: host root: %w", number, err)
	}

	cmd := exec.Command("go", "run", "./services/calc", "halve", strconv.Itoa(number))
	cmd.Dir = hostRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("halve %d: %w: %s", number, err, strings.TrimSpace(stderr.String()))
	}

	result, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	if err != nil {
		return fmt.Errorf("halve %d: integer stdout %q: %w", number, stdout.String(), err)
	}
	state.Result = result
	return nil
}
