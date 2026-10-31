package steps

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

//truebdd:concept when "operation"
func concept40523ceb26a25e37e78222ac562623f48c38b52711218d94fe5cadea337eebba(state *State, args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("when operation: expected 3 captures (exact, operation, number), got %d", len(args))
	}
	exact, operation, number := args[0], args[1], args[2]

	// The Calculator CLI document declares both commands as
	// "go run ./services/calc <command> ... from the host root"; the suite's
	// tests run in tests/calc, so the host root is two levels up.
	const hostRoot = "../.."

	var command string
	switch operation {
	case "halves":
		command = "halve"
	case "doubles":
		command = "double"
	default:
		return fmt.Errorf("when operation: the step names operation %q, which is neither halves nor doubles", operation)
	}

	cmdArgs := []string{"run", "./services/calc", command}
	if exact != "" {
		if command != "halve" {
			return fmt.Errorf("when operation: the Calculator CLI declares --exact only for halve, not for %s", command)
		}
		cmdArgs = append(cmdArgs, "--exact")
	}
	cmdArgs = append(cmdArgs, number)

	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = hostRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("when operation: go %s: %w: %s", strings.Join(cmdArgs, " "), err, strings.TrimSpace(stderr.String()))
	}

	state.Output = strings.TrimRight(stdout.String(), "\r\n")
	return nil
}
