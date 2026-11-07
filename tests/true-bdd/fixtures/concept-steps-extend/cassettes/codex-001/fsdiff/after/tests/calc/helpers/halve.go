package helpers

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// HostRoot is the host root the calculator CLI is run from, relative to
// the suite's test working directory (tests/calc).
const HostRoot = "../.."

// Halve runs the declared `halve` command as a process, passing value
// verbatim as its single required argument, and returns the integer the
// command printed on stdout.
//
// The calculator alone decides the halved number: Halve only carries the
// step's whole number to the command and reads back what the command
// printed, so nothing here computes or adjusts a half.
func Halve(value string) (int, error) {
	cmd := exec.Command("go", "run", "./services/calc", "halve", value)
	cmd.Dir = HostRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return 0, fmt.Errorf("halve %s: %w: %s", value, err, message)
		}
		return 0, fmt.Errorf("halve %s: %w", value, err)
	}

	printed := strings.TrimSpace(stdout.String())
	result, err := strconv.Atoi(printed)
	if err != nil {
		return 0, fmt.Errorf("halve %s: stdout %q is not an integer: %w", value, printed, err)
	}
	return result, nil
}
