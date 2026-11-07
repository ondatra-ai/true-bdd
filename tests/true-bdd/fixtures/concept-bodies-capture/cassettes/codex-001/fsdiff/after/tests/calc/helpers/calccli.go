// Package helpers holds the code the calc suite's step definitions
// share: how to reach the calculator through its declared command-line
// interface, and nothing about what the calculator should answer.
package helpers

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// HostRoot is the directory the Calculator CLI is documented to run
// from, written relative to a suite test's working directory
// (tests/calc, per the registry path the scenarios harness uses).
const HostRoot = "../.."

// RunHalve runs the declared `halve` command as a process, with value
// as its one integer argument, and returns the integer the command
// printed on stdout.
//
// The helper performs no arithmetic: the number it returns is whatever
// the calculator reported, so a wrong answer from the calculator comes
// back unchanged for the observing step to compare.
func RunHalve(value string) (int, error) {
	cmd := exec.Command("go", "run", "./services/calc", "halve", value)
	cmd.Dir = HostRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("halve %s: %w: %s", value, err, strings.TrimSpace(stderr.String()))
	}

	printed := strings.TrimSpace(stdout.String())
	result, err := strconv.Atoi(printed)
	if err != nil {
		return 0, fmt.Errorf("halve %s: stdout %q is not an integer: %w", value, printed, err)
	}
	return result, nil
}
