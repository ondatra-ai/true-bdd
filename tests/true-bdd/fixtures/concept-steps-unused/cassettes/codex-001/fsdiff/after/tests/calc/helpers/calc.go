package helpers

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// HostRoot is the host root the calculator CLI is run from, relative to a
// suite test's working directory (tests/calc) — the same relative base the
// suite already uses for ../../docs/scenarios.yaml.
const HostRoot = "../.."

// Halve runs the declared `halve` command as a process with value as its
// value argument and returns the integer the command wrote on stdout.
//
// It never halves anything itself: the product's stdout is the only source of
// the returned number. Per docs/architecture/calc-cli.yaml, halve is run as
// `go run ./services/calc halve VALUE` from the host root, exit 0 prints the
// halved integer and a newline, exit 1 means invalid arguments.
func Halve(value int) (int, error) {
	cmd := exec.Command("go", "run", "./services/calc", "halve", strconv.Itoa(value))
	cmd.Dir = HostRoot

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("halve %d: %w: %s", value, err, strings.TrimSpace(stderr.String()))
	}

	out := strings.TrimSpace(stdout.String())
	result, err := strconv.Atoi(out)
	if err != nil {
		return 0, fmt.Errorf("halve %d: stdout %q is not an integer: %w", value, out, err)
	}

	return result, nil
}
