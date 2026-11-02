package steps

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "doubling"
func concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b(state *State, args []string) error {
	number, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("the Owner doubles %q: not a whole number: %w", args[0], err)
	}
	// The suite's Go test runs in tests/calc, so the host root is two levels
	// up: the same root the scenarios package uses for docs/scenarios.yaml
	// and the boundary test uses for tmp/calc-calls.log.
	const hostRoot = "../.."
	cmd := exec.Command("go", "run", "./services/calc", "double", strconv.Itoa(number))
	cmd.Dir = hostRoot
	stdout, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("go run ./services/calc double %d: %w: %s", number, err, strings.TrimSpace(string(exit.Stderr)))
		}
		return fmt.Errorf("go run ./services/calc double %d: %w", number, err)
	}
	doubled, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil {
		return fmt.Errorf("go run ./services/calc double %d: stdout %q is not an integer: %w", number, string(stdout), err)
	}
	state.Value = doubled
	return nil
}
