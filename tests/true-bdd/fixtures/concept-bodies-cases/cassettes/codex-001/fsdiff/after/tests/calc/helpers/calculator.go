// Package helpers holds the code the calc suite's step definitions share.
//
// The calculator is reached only as the declared CLI of
// docs/architecture/calc-cli.yaml: `go run ./services/calc halve [--exact] VALUE`
// and `go run ./services/calc double VALUE`, run as a process from the host root.
// Nothing here halves or doubles a number; the number travels to the command as
// the step captured it, and the command's stdout travels back untouched but for
// the single line terminator the CLI document says it ends with.
package helpers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// The words a step's captures use, and the CLI names they select.
const (
	// OperationHalves and OperationDoubles are the two alternatives the
	// operation capture can hold.
	OperationHalves  = "halves"
	OperationDoubles = "doubles"

	// CommandHalve and CommandDouble are the commands declared in the
	// calculator's CLI document.
	CommandHalve  = "halve"
	CommandDouble = "double"

	// ExactlyWord is the optional word the exact capture holds when the step
	// says it; ExactFlag is the flag halve declares for it.
	ExactlyWord = "exactly"
	ExactFlag   = "--exact"

	// ServicePackage is the package the declared commands are run from, as
	// named by the CLI document, relative to the host root.
	ServicePackage = "./services/calc"
)

// runTimeout bounds one calculator run so a wedged process reports itself
// instead of hanging the suite.
const runTimeout = 2 * time.Minute

// CommandForOperation turns the operation the step names into the command the
// CLI document declares for it.
func CommandForOperation(operation string) (string, error) {
	switch operation {
	case OperationHalves:
		return CommandHalve, nil
	case OperationDoubles:
		return CommandDouble, nil
	default:
		return "", fmt.Errorf(
			"operation %q names no declared command: the calculator CLI declares %q and %q only",
			operation, OperationHalves, OperationDoubles,
		)
	}
}

// CalculatorArgs builds the argument list for one run of the calculator: the
// command the operation names, --exact when the step said "exactly", and the
// step's number as the VALUE argument, passed on as captured.
func CalculatorArgs(operation, exact, number string) ([]string, error) {
	command, err := CommandForOperation(operation)
	if err != nil {
		return nil, err
	}

	argv := []string{command}

	switch exact {
	case "":
		// The step did not say "exactly", so no flag is passed.
	case ExactlyWord:
		if command != CommandHalve {
			return nil, fmt.Errorf(
				"the calculator CLI declares the %s flag on %q only, so %q cannot be run exactly",
				ExactFlag, CommandHalve, command,
			)
		}
		argv = append(argv, ExactFlag)
	default:
		return nil, fmt.Errorf(
			"exact capture %q is neither empty nor %q", exact, ExactlyWord,
		)
	}

	return append(argv, number), nil
}

// HostRoot is the directory the declared commands are run from: the nearest
// ancestor of the suite's working directory holding a go.mod, which is the host
// root the CLI document names `./services/calc` relative to.
func HostRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("locating the host root: %w", err)
	}
	for {
		info, statErr := os.Stat(filepath.Join(dir, "go.mod"))
		if statErr == nil && !info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf(
				"no go.mod above the suite's working directory, so the host root to run %s from is unknown",
				ServicePackage,
			)
		}
		dir = parent
	}
}

// TrimLine drops the single trailing line terminator the CLI document says each
// command's stdout ends with, and nothing else.
func TrimLine(out string) string {
	return strings.TrimSuffix(strings.TrimSuffix(out, "\n"), "\r")
}

// RunCalculator runs the calculator as a process from the host root with the
// given command arguments and returns its stdout. The inherited environment is
// left in place, so the Go build and temporary-directory settings of the run
// carry through.
func RunCalculator(argv ...string) (string, error) {
	root, err := HostRoot()
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	full := append([]string{"run", ServicePackage}, argv...)
	cmd := exec.CommandContext(ctx, "go", full...)
	cmd.Dir = root

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	output := TrimLine(stdout.String())
	if runErr == nil {
		return output, nil
	}

	printed := strings.TrimSpace(stderr.String())
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return output, fmt.Errorf(
			"go %s in %s did not finish within %s; stderr: %s",
			strings.Join(full, " "), root, runTimeout, printed,
		)
	}
	return output, fmt.Errorf(
		"go %s in %s: %w; stderr: %s",
		strings.Join(full, " "), root, runErr, printed,
	)
}
