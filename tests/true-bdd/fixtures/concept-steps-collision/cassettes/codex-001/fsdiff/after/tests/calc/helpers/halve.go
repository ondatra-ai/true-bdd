// Package helpers holds the code the calc suite's step definitions
// share. It carries a step's values in to the calculator and reads the
// calculator's own answer back out; it never works an answer out here.
package helpers

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// HostRoot is the path from a calc suite test's working directory
// (tests/calc) to the host root, the directory the calculator CLI is
// declared to run from.
const HostRoot = "../.."

// Halve runs the calculator's declared halve command as a process,
// passing value verbatim as its VALUE argument, and returns the integer
// the command writes to stdout. The halving is the calculator's work:
// this reports what the process answered and nothing else.
//
// Declared in docs/architecture/calc-cli.yaml: `go run ./services/calc
// halve VALUE` run from the host root, whose stdout is the halved
// integer and a newline, exit 0 on success and 1 on invalid arguments.
func Halve(value string) (int, error) {
	cmd := exec.Command("go", "run", "./services/calc", "halve", value)
	cmd.Dir = HostRoot

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.Output()
	if err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return 0, fmt.Errorf("run halve %s: %w: %s", value, err, message)
		}
		return 0, fmt.Errorf("run halve %s: %w", value, err)
	}

	answer := strings.TrimSpace(string(stdout))
	result, err := strconv.Atoi(answer)
	if err != nil {
		return 0, fmt.Errorf("run halve %s: stdout %q is not an integer: %w", value, answer, err)
	}
	return result, nil
}
