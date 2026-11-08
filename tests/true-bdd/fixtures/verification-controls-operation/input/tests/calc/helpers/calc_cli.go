// Package helpers holds what the calc suite's step definitions share: the
// host root the calculator's declared commands run from, the translation
// from a step's wording to a declared command name, and the single process
// spawn that keeps a command's stdout. What a half or a double is stays the
// calculator's business; nothing here computes either.
package helpers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ServicePackage is the calculator's package path as docs/architecture/calc-cli.yaml
// states it — `go run ./services/calc <command> …`, relative to the host root.
const ServicePackage = "./services/calc"

// ExactFlag is the flag the declared halve command takes so an odd value's
// half is kept instead of rounded toward zero. The document declares it on
// halve alone.
const ExactFlag = "--exact"

// CommandTimeout bounds one invocation, so a calculator that never exits
// fails the scenario that ran it instead of hanging the suite. Generous
// enough for `go run` to compile the service on a cold cache.
const CommandTimeout = 2 * time.Minute

// hostRootMarker is the file that marks the host root. `./services/calc` is
// a module-relative package path, so the root the document means is the
// module's own directory, found by walking up from the test's directory
// rather than hard-coded as a count of `..` segments.
const hostRootMarker = "go.mod"

// operationCaptures is how many captures the when "operation" step hands a
// definition: the optional word "exactly", the operation named, and the
// number.
const operationCaptures = 3

// exactlyWord is that first capture's value when the step says the Owner
// halves exactly; it is empty when the step does not say so.
const exactlyWord = "exactly"

// ErrUnknownOperation signals step wording that names no command the
// architectural spec declares for this service.
var ErrUnknownOperation = errors.New("the step names no declared calculator command")

// ErrExactNotDeclared signals a request for --exact on a command whose
// declaration has no such flag. Refused here rather than passed through,
// since the interface document, not the step, settles which flags exist.
var ErrExactNotDeclared = errors.New("only the declared halve command takes --exact")

// ErrHostRootNotFound signals that no module root sits at or above the
// working directory, so there is nowhere to run the declared command from.
var ErrHostRootNotFound = errors.New("no go.mod at or above the working directory")

// ErrCaptureCount signals captures that do not match the pattern the step
// was registered with, so a definition never reads a capture it was never
// handed.
var ErrCaptureCount = errors.New("unexpected number of step captures")

// CommandFor translates the operation a step names into the command the
// architectural spec declares under that name.
func CommandFor(operation string) (string, error) {
	switch operation {
	case "halves":
		return "halve", nil

	case "doubles":
		return "double", nil

	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownOperation, operation)
	}
}

// OperationFromStep reads the when "operation" step's captures — the
// optional word "exactly", the operation the step names, and its whole
// number — and returns the declared command to run, whether --exact goes
// with it, and the VALUE argument to pass. It only translates the step's
// wording; the halving and the doubling stay the calculator's business.
func OperationFromStep(args []string) (command string, exact bool, value string, err error) {
	if len(args) != operationCaptures {
		return "", false, "", fmt.Errorf("%w: got %d, want %d: %v",
			ErrCaptureCount, len(args), operationCaptures, args)
	}

	command, err = CommandFor(args[1])
	if err != nil {
		return "", false, "", err
	}

	return command, args[0] == exactlyWord, args[2], nil
}

// HostRoot returns the directory the declared commands run from: the
// nearest ancestor of the working directory holding go.mod.
func HostRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("locating the host root: %w", err)
	}

	for {
		_, statErr := os.Stat(filepath.Join(dir, hostRootMarker))
		if statErr == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("%w: %s", ErrHostRootNotFound, dir)
		}

		dir = parent
	}
}

// RunCalc runs one declared calculator command as a process from the host
// root, with value as its VALUE argument and --exact when asked, and
// returns its stdout exactly as written — trailing newline included, since
// trimming it is the observing step's own clause.
func RunCalc(command string, exact bool, value string) (string, error) {
	if exact && command != "halve" {
		return "", fmt.Errorf("%w: %s", ErrExactNotDeclared, command)
	}

	root, err := HostRoot()
	if err != nil {
		return "", err
	}

	args := []string{"run", ServicePackage, command}
	if exact {
		args = append(args, ExactFlag)
	}

	args = append(args, value)

	ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
	defer cancel()

	var stdout, stderr strings.Builder

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = root
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("go %s (in %s): %w%s",
			strings.Join(args, " "), root, err, stderrTail(stderr.String()))
	}

	return stdout.String(), nil
}

// stderrTail renders what the process said on stderr as a suffix for a
// failure message, and nothing at all when it said nothing.
func stderrTail(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}

	return ": " + trimmed
}

