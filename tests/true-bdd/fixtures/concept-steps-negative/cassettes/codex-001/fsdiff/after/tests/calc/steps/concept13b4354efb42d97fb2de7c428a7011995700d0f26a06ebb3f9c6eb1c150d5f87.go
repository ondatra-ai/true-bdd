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
func concept13b4354efb42d97fb2de7c428a7011995700d0f26a06ebb3f9c6eb1c150d5f87(state *State, args []string) error {
	number := args[0]

	// The declared halve command runs from the host root: walk up from the
	// suite's working directory to the directory holding go.mod.
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("halve %s: locating the host root: %w", number, err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(root, "go.mod")); statErr == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			return fmt.Errorf("halve %s: no host root with go.mod above the working directory", number)
		}
		root = parent
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("go", "run", "./services/calc", "halve", number)
	cmd.Dir = root
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("halve %s: %w: %s", number, err, strings.TrimSpace(stderr.String()))
	}

	printed := strings.TrimSpace(stdout.String())
	halved, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("halve %s: stdout %q is not an integer: %w", number, printed, err)
	}

	state.Result = halved
	return nil
}
