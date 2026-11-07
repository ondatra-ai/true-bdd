// Package helpers holds the code the calc suite's step definitions share.
//
// It reaches the calculator only through its declared CLI (see
// docs/architecture/calc-cli.yaml) and never works out a halved value
// itself: every number it returns is the command's own stdout.
package helpers

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// HostRoot is the host root the calculator CLI is documented to run from,
// relative to the calc suite's working directory (tests/calc).
const HostRoot = "../.."

// Halve runs the declared `halve` command as a process, passing value
// through unchanged as the command's `value` argument, and returns the
// integer the command writes on stdout.
//
// value is handed over verbatim, so a leading minus sign reaches the
// command as part of its argument; `go run` forwards everything after the
// package path to the program.
func Halve(value string) (int, error) {
	cmd := exec.Command("go", "run", "./services/calc", "halve", value)
	cmd.Dir = HostRoot
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) > 0 {
			return 0, fmt.Errorf("halve %s: %w: %s", value, err, strings.TrimSpace(string(exit.Stderr)))
		}
		return 0, fmt.Errorf("halve %s: %w", value, err)
	}
	text := strings.TrimSpace(string(out))
	result, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("halve %s: stdout %q is not an integer: %w", value, text, err)
	}
	return result, nil
}
