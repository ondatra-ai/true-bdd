package steps

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

//truebdd:concept when "halving"
func concept6ac0aa0a415cecc00e2fb53aee3eb1fb378819d8a71108ae58475b7f0ebbe4ea(state *State, args []string) error {
	value := args[0]
	if _, err := strconv.Atoi(value); err != nil {
		return fmt.Errorf("halve: step number %q is not a whole number: %w", value, err)
	}

	// The declared halve command runs from the host root; the suite runs from
	// tests/calc, so climb to the directory holding the module's go.mod.
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("halve %s: locating the host root: %w", value, err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(root, "go.mod")); statErr == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			return fmt.Errorf("halve %s: no host root with go.mod above the suite directory", value)
		}
		root = parent
	}

	cmd := exec.Command("go", "run", "./services/calc", "halve", value)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		return fmt.Errorf("halve %s: %w: %s", value, runErr, strings.TrimSpace(stderr.String()))
	}

	halved, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	if err != nil {
		return fmt.Errorf("halve %s: stdout %q is not an integer: %w", value, stdout.String(), err)
	}

	state.Result = halved
	return nil
}
