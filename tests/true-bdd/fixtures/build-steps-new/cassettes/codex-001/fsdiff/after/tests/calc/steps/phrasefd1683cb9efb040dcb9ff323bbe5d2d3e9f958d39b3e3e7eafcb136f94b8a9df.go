package steps

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func phrasefd1683cb9efb040dcb9ff323bbe5d2d3e9f958d39b3e3e7eafcb136f94b8a9df(state *State, args []string) error {
	// The declared CLI runs `go run ./services/calc double VALUE` from the
	// host root; the suite's tests execute inside tests/calc, so the host
	// root is two levels up (same relative base the suite uses for
	// ../../docs/scenarios.yaml).
	const hostRoot = "../.."

	operand := state.Value

	cmd := exec.Command("go", "run", "./services/calc", "double", strconv.Itoa(operand))
	cmd.Dir = hostRoot

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run double %d: %w: %s", operand, err, strings.TrimSpace(stderr.String()))
	}

	doubled, convErr := strconv.Atoi(strings.TrimSpace(stdout.String()))
	if convErr != nil {
		return fmt.Errorf("double %d stdout %q: %w", operand, stdout.String(), convErr)
	}

	state.Value = doubled
	return nil
}
