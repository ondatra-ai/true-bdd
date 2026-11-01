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
	value := args[0]
	if _, err := strconv.Atoi(value); err != nil {
		return fmt.Errorf("the Owner halves %q: not a whole number: %w", value, err)
	}

	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("the Owner halves %s: working directory: %w", value, err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(root, "go.mod")); statErr == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			return fmt.Errorf("the Owner halves %s: host root with go.mod not found above the working directory", value)
		}
		root = parent
	}

	command := exec.Command("go", "run", "./services/calc", "halve", value)
	command.Dir = root
	var stdout, stderr strings.Builder
	command.Stdout = &stdout
	command.Stderr = &stderr
	if runErr := command.Run(); runErr != nil {
		return fmt.Errorf("the Owner halves %s: %s: %w: %s",
			value,
			strings.Join(command.Args, " "),
			runErr,
			strings.TrimSpace(stderr.String()),
		)
	}

	printed := strings.TrimSpace(stdout.String())
	halved, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("the Owner halves %s: stdout %q is not an integer: %w", value, printed, err)
	}

	state.Result = halved
	return nil
}
