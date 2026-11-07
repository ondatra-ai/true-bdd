package helpers

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// HostRoot is the path from a calc suite test's working directory (tests/calc)
// to the host root, which is where the declared calculator CLI is run from.
const HostRoot = "../.."

// RunHalve runs the declared halve command as a process, passing value through
// unchanged as the command's value argument, and returns the integer carried on
// the command's stdout. It never halves value itself: the returned integer is
// only what the command printed.
func RunHalve(value string) (int, error) {
	cmd := exec.Command("go", "run", "./services/calc", "halve", value)
	cmd.Dir = HostRoot
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("halve %s: %w", value, err)
	}
	result, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("halve %s: stdout %q is not an integer: %w", value, string(out), err)
	}
	return result, nil
}
