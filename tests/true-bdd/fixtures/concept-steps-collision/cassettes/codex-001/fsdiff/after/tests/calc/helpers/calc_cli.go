// Package helpers holds code shared by the calc suite's step definitions.
//
// It reaches the calculator only through its declared command-line
// interface (docs/architecture/calc-cli.yaml) and never reproduces the
// calculator's arithmetic: every number a step compares against comes
// back from the product's own stdout.
package helpers

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// hostRoot is the path from the suite's test working directory
// (tests/calc) to the host root, which is where the declared halve
// command is run from.
const hostRoot = "../.."

// Halve runs the declared halve command as a process, passing value
// verbatim as its single value argument, and returns the integer the
// command wrote to stdout.
//
// value is handed to the process unchanged, so a leading minus sign
// reaches the command as part of its argument. The halving and the
// rounding are the product's work; this helper only starts the process
// and reads back what it printed.
func Halve(value string) (int, error) {
	cmd := exec.Command("go", "run", "./services/calc", "halve", value)
	cmd.Dir = hostRoot
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) > 0 {
			return 0, fmt.Errorf("halve %s: %w: %s", value, err, strings.TrimSpace(string(exit.Stderr)))
		}
		return 0, fmt.Errorf("halve %s: %w", value, err)
	}
	printed := strings.TrimSpace(string(out))
	result, err := strconv.Atoi(printed)
	if err != nil {
		return 0, fmt.Errorf("halve %s: stdout %q is not an integer: %w", value, printed, err)
	}
	return result, nil
}
