package steps

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//truebdd:concept when "operation"
func concept40523ceb26a25e37e78222ac562623f48c38b52711218d94fe5cadea337eebba(state *State, args []string) error {
	exact, operation, number := args[0], args[1], args[2]

	var command string
	switch operation {
	case "halves":
		command = "halve"
	case "doubles":
		command = "double"
	default:
		return fmt.Errorf("the step names no declared operation: %q", operation)
	}

	runArgs := []string{"run", "./services/calc", command}
	if exact != "" {
		if command != "halve" {
			return fmt.Errorf("the interface declares --exact for halve only, not for %s", command)
		}
		runArgs = append(runArgs, "--exact")
	}
	runArgs = append(runArgs, number)

	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("find the host root: %w", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(root, "go.mod")); statErr == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			return fmt.Errorf("find the host root: no go.mod at or above the suite's working directory")
		}
		root = parent
	}

	cmd := exec.Command("go", runArgs...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		return fmt.Errorf("go %s: %w: %s", strings.Join(runArgs, " "), runErr, strings.TrimSpace(stderr.String()))
	}

	state.Output = strings.TrimRight(stdout.String(), "\n")
	return nil
}
