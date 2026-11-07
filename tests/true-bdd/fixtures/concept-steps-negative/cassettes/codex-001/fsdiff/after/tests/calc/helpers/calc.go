// Package helpers holds code the calc suite's step definitions share:
// running the calculator CLI the way the concept bindings describe and
// reading back what it printed. Nothing here computes a calculator
// answer; the product's process is the only source of a result.
package helpers

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// commandTimeout bounds one calculator run so a hung process fails the
// step instead of the whole suite.
const commandTimeout = 2 * time.Minute

// HostRoot is the repository root as seen from a suite test's working
// directory (tests/calc), matching the relative paths the generated
// scenarios harness already uses.
func HostRoot() string { return filepath.Join("..", "..") }

// Halve runs `go run ./services/calc halve VALUE` from the host root with
// value as the single value argument, passed verbatim so a leading minus
// sign reaches the command unchanged, and returns the integer the command
// printed on stdout.
func Halve(value string) (int, error) {
	stdout, err := RunCalc("halve", value)
	if err != nil {
		return 0, err
	}
	return parseInt("halve", value, stdout)
}

// RunCalc runs the calculator CLI with args from the host root and returns
// its stdout. A non-zero exit is reported with the command's stderr.
func RunCalc(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	argv := append([]string{"run", "./services/calc"}, args...)
	cmd := exec.CommandContext(ctx, "go", argv...)
	cmd.Dir = HostRoot()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go run ./services/calc %s (in %s): %w: %s",
			strings.Join(args, " "), cmd.Dir, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// parseInt reads the command's integer stdout, trimming the trailing
// newline the CLI document describes.
func parseInt(command, value, stdout string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(stdout))
	if err != nil {
		return 0, fmt.Errorf("calc %s %s: stdout %q is not an integer: %w",
			command, value, stdout, err)
	}
	return n, nil
}
