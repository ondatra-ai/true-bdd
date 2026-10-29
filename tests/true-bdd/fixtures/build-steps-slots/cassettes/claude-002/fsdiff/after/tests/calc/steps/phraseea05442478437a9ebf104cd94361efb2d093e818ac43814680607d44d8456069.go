package steps

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func phraseea05442478437a9ebf104cd94361efb2d093e818ac43814680607d44d8456069(state *State, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("the command %q doubles %v: expected an operation and an operand, got %d argument(s)", "double", args, len(args))
	}
	operation := args[0]
	operand := args[1]
	if _, err := strconv.Atoi(operand); err != nil {
		return fmt.Errorf("operand %q is not an integer: %w", operand, err)
	}

	const hostRoot = "../.."
	const commandPackage = "./services/calc"

	cmd := exec.Command("go", "run", commandPackage, operation, operand)
	cmd.Dir = hostRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go run %s %s %s: %w: %s", commandPackage, operation, operand, err, strings.TrimSpace(stderr.String()))
	}

	observed, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	if err != nil {
		return fmt.Errorf("stdout %q of %s %s is not an integer: %w", stdout.String(), operation, operand, err)
	}
	state.Value = observed
	return nil
}
