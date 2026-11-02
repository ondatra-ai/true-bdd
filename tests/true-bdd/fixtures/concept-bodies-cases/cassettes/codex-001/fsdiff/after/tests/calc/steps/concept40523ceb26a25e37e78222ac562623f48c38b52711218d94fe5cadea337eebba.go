package steps

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

//truebdd:concept when "operation"
func concept40523ceb26a25e37e78222ac562623f48c38b52711218d94fe5cadea337eebba(state *State, args []string) error {
	exact, operation, value := args[0], args[1], args[2]

	// The declared CLI is run from the host root. The suite's tests run
	// with tests/calc as their working directory, the same place the
	// registry is reached from as "../../docs/scenarios.yaml".
	const hostRoot = "../.."
	const mainPackage = "./services/calc"

	var command string
	switch operation {
	case "halves":
		command = "halve"
	case "doubles":
		command = "double"
	default:
		return fmt.Errorf("the step names no declared operation: %q", operation)
	}

	runArgs := []string{"run", mainPackage, command}
	if exact != "" {
		if command != "halve" {
			return fmt.Errorf("the declared %s command takes no --exact flag, so the step cannot %s %s", command, exact, operation)
		}
		runArgs = append(runArgs, "--exact")
	}
	runArgs = append(runArgs, value)

	cmd := exec.Command("go", runArgs...)
	cmd.Dir = hostRoot

	stdout, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("go %s exited %d: %s", strings.Join(runArgs, " "), exit.ExitCode(), strings.TrimSpace(string(exit.Stderr)))
		}
		return fmt.Errorf("running go %s: %w", strings.Join(runArgs, " "), err)
	}

	state.Output = string(stdout)
	return nil
}
