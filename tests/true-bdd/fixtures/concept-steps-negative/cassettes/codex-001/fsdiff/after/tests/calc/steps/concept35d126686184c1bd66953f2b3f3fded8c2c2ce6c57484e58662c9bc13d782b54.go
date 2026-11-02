package steps

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "halving"
func concept35d126686184c1bd66953f2b3f3fded8c2c2ce6c57484e58662c9bc13d782b54(state *State, args []string) error {
	// The calc CLI document declares halve as: run
	// `go run ./services/calc halve VALUE` from the host root. `go test`
	// runs in the suite's package directory, tests/calc, so the host root
	// is two levels up — the same relative root scenarios.go uses for the
	// registry.
	const hostRoot = "../.."

	role, number := args[0], args[1]

	// The command's declared parameter is an integer, so a capture that is
	// not one is this step's own fault and is reported as such rather than
	// handed to the process.
	if _, err := strconv.Atoi(number); err != nil {
		return fmt.Errorf("the %s halves %q: step's number is not a whole number: %w", role, number, err)
	}

	command := exec.Command("go", "run", "./services/calc", "halve", number)
	command.Dir = hostRoot

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		return fmt.Errorf("the %s halves %s: go run ./services/calc halve %s in %s: %w: %s",
			role, number, number, hostRoot, err, strings.TrimSpace(stderr.String()))
	}

	// "Keep its integer stdout as the result": the halved value is whatever
	// the process printed, never anything this step works out.
	halved, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	if err != nil {
		return fmt.Errorf("the %s halves %s: halve stdout %q is not an integer: %w",
			role, number, stdout.String(), err)
	}

	state.Result = halved

	return nil
}
