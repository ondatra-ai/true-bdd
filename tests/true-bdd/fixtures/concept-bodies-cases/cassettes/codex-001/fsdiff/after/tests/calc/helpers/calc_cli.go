// Package helpers holds the code the calc suite's step definitions share:
// one place that knows how to run the calculator's declared CLI commands as
// processes and hand their stdout back to a step. Nothing here computes a
// half or a double; the process the calculator owns does that.
package helpers

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// CommandHalve and CommandDouble are the command names the calculator's
	// CLI document declares.
	CommandHalve  = "halve"
	CommandDouble = "double"

	// ExactFlag is the flag the CLI document declares on halve, which keeps
	// an odd VALUE's half instead of rounding it toward zero.
	ExactFlag = "--exact"

	// OperationHalves and OperationDoubles are the operation words a step
	// names.
	OperationHalves  = "halves"
	OperationDoubles = "doubles"

	// servicePackage is the package the CLI document says to run.
	servicePackage = "./services/calc"

	// runTimeout bounds one run of the calculator so a wedged process fails
	// the step instead of hanging the suite.
	runTimeout = 2 * time.Minute
)

// hostRoot is the directory the CLI document says to run the calculator
// from, relative to the suite test binary's working directory (tests/calc).
var hostRoot = filepath.Join("..", "..")

// OperationCommand maps the operation word a step names to the calculator
// command this concept's binding declares for it.
func OperationCommand(operation string) (string, error) {
	switch operation {
	case OperationHalves:
		return CommandHalve, nil
	case OperationDoubles:
		return CommandDouble, nil
	default:
		return "", fmt.Errorf("unknown operation %q: the calculator declares only %q and %q", operation, CommandHalve, CommandDouble)
	}
}

// RunCalc runs one declared calculator command as a process from the host
// root, as the CLI document describes, and returns its stdout verbatim,
// trailing newline included. A non-zero exit becomes an error carrying the
// command line and the process's stderr.
func RunCalc(command string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	argv := append([]string{"run", servicePackage, command}, args...)
	cmd := exec.CommandContext(ctx, "go", argv...)
	cmd.Dir = hostRoot

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = "no stderr"
		}
		return "", fmt.Errorf("go %s (in %s): %w: %s", strings.Join(argv, " "), hostRoot, err, detail)
	}

	return stdout.String(), nil
}
