// Package helpers runs the calculator's public CLI for the calc suite's step
// definitions. These helpers only start the declared command and carry back
// what the process printed on stdout; every arithmetic decision stays inside
// the calculator, which is the subject under test.
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

// HostRoot is the host root as seen from a Go test's working directory, which
// is the suite package directory tests/calc. docs/architecture/calc-cli.yaml
// spells the command as run "from the host root".
var HostRoot = filepath.Join("..", "..")

// CalcPackage is the Go package the calculator CLI is run from, exactly as the
// CLI document spells it: go run ./services/calc <command> ...
const CalcPackage = "./services/calc"

// CommandTimeout bounds one calculator run, so a wedged process fails the step
// instead of hanging the suite.
const CommandTimeout = 2 * time.Minute

// RunCalc runs `go run ./services/calc <command> <args...>` from the host root
// and returns its stdout. A non-zero exit, per the CLI document's '1' response,
// becomes the step's error together with anything the command wrote on stderr.
func RunCalc(command string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
	defer cancel()

	argv := append([]string{"run", CalcPackage, command}, args...)
	cmd := exec.CommandContext(ctx, "go", argv...)
	cmd.Dir = HostRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %w%s", describe(command, args), err, stderrSuffix(stderr.String()))
	}
	return stdout.String(), nil
}

// RunCalcInt runs the command and reads its stdout as the single integer the
// CLI document gives for an integer response, ignoring the trailing newline.
func RunCalcInt(command string, args ...string) (int, error) {
	out, err := RunCalc(command, args...)
	if err != nil {
		return 0, err
	}
	text := strings.TrimSpace(out)
	value, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("%s: stdout %q is not an integer", describe(command, args), text)
	}
	return value, nil
}

// HalveCommand is the command the halving concept's binding references: the
// `halve` command of docs/architecture/calc-cli.yaml, which takes one integer
// `value` argument and prints the answer on stdout.
const HalveCommand = "halve"

// Halve runs the declared halve command as a process with value as its `value`
// argument and returns the integer the calculator printed on stdout. The
// halving and its rounding are the calculator's own work: this helper only
// starts the declared command and reads back what the process printed.
func Halve(value string) (int, error) {
	return RunCalcInt(HalveCommand, value)
}

func describe(command string, args []string) string {
	parts := append([]string{"go", "run", CalcPackage, command}, args...)
	return strings.Join(parts, " ")
}

func stderrSuffix(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return ": " + text
}

