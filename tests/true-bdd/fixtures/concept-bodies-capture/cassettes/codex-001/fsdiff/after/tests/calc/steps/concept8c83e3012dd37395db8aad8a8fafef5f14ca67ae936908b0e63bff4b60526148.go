package steps

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "halving"
func concept8c83e3012dd37395db8aad8a8fafef5f14ca67ae936908b0e63bff4b60526148(state *State, args []string) error {
	// docs/architecture/calc-cli.yaml, command `halve`: run
	// `go run ./services/calc halve VALUE` from the host root; stdout is the
	// halved integer and a newline. The tests run from tests/calc, so the host
	// root is two levels up, the same root the suite reads its registry from.
	const hostRoot = "../.."

	value, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("halving: %q is not a whole number: %w", args[0], err)
	}

	cmd := exec.Command("go", "run", "./services/calc", "halve", strconv.Itoa(value))
	cmd.Dir = hostRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("halve %d: %w: %s", value, err, strings.TrimSpace(stderr.String()))
	}

	printed := strings.TrimSpace(stdout.String())
	halved, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("halve %d: stdout %q is not an integer: %w", value, printed, err)
	}

	state.Result = halved
	return nil
}
