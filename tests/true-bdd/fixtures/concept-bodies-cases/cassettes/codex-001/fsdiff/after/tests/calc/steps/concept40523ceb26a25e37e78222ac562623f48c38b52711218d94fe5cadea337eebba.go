package steps

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

//truebdd:concept when "operation"
func concept40523ceb26a25e37e78222ac562623f48c38b52711218d94fe5cadea337eebba(state *State, args []string) error {
	exactly, operation, number := args[0], args[1], args[2]

	// The suite's tests run in tests/calc, so the host root the declared CLI
	// documents its commands from is two directories up.
	const hostRoot = "../.."
	const servicePackage = "./services/calc"

	var command string
	switch operation {
	case "halves":
		command = "halve"
	case "doubles":
		command = "double"
	default:
		return fmt.Errorf("the step names no declared operation: %q", operation)
	}

	argv := []string{"run", servicePackage, command}
	if exactly != "" {
		if command != "halve" {
			return fmt.Errorf("the declared %s command takes no --exact flag", command)
		}
		argv = append(argv, "--exact")
	}
	argv = append(argv, number)

	cmd := exec.Command("go", argv...)
	cmd.Dir = hostRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go %s in %s: %w: %s", strings.Join(argv, " "), hostRoot, err, strings.TrimSpace(stderr.String()))
	}

	// Keep the command's stdout as the output, without the trailing newline the
	// declared interface says it prints after the value.
	state.Output = strings.TrimRight(stdout.String(), "\r\n")
	return nil
}
