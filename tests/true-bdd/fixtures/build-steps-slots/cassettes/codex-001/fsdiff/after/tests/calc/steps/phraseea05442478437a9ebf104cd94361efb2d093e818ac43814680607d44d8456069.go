package steps

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func phraseea05442478437a9ebf104cd94361efb2d093e818ac43814680607d44d8456069(state *State, args []string) error {
	const (
		hostRoot = "../.."
		pkgPath  = "./services/calc"
	)
	if len(args) < 2 {
		return fmt.Errorf("expected an operation and an operand, got %d slot(s): %q", len(args), args)
	}
	operation := args[0]
	operand, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("operand %q is not an integer: %w", args[1], err)
	}

	cmd := exec.Command("go", "run", pkgPath, operation, strconv.Itoa(operand))
	cmd.Dir = hostRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go run %s %s %d: %w: %s", pkgPath, operation, operand, err, strings.TrimSpace(stderr.String()))
	}

	printed := strings.TrimSpace(stdout.String())
	observed, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("stdout %q from %s %s is not an integer: %w", printed, pkgPath, operation, err)
	}
	state.Value = observed
	return nil
}
