package helpers

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ServicePackage is the Go package the declared calculator CLI names:
// "go run ./services/calc <command> ... from the host root".
const ServicePackage = "./services/calc"

// ExactFlag is the flag the declared halve command takes, the only command
// of the two that declares it, to keep an odd value's half.
const ExactFlag = "--exact"

// HalveCommand and DoubleCommand are the two commands the declared CLI has.
const (
	HalveCommand  = "halve"
	DoubleCommand = "double"
)

// Operation returns the declared CLI command, and the flags that command
// declares, for the operation phrase a step names. "halves" and "exactly
// halves" both name the declared halve command, the second with the --exact
// flag halve alone declares; "doubles" names the declared double command,
// which declares no flag. The helper only translates the step's phrase into
// the declared invocation; the calculator alone works out what it prints.
func Operation(operation string) (string, []string, error) {
	switch operation {
	case "halves":
		return HalveCommand, nil, nil
	case "exactly halves":
		return HalveCommand, []string{ExactFlag}, nil
	case "doubles":
		return DoubleCommand, nil, nil
	default:
		return "", nil, fmt.Errorf("the declared calculator CLI has no command for the operation %q", operation)
	}
}

// HostRoot is the directory the declared CLI is run from: the root of the
// host module, found by walking up from the test's working directory to the
// go.mod that sits beside docs/ and services/.
func HostRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("locate the host root: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("locate the host root: no go.mod at or above the test's working directory")
		}
		dir = parent
	}
}

// Run runs the declared calculator command as a process from the host root
// and returns its stdout verbatim, trailing newline included. It carries the
// step's arguments in and the command's stdout back out; the calculator
// computes the answer.
func Run(command string, args ...string) (string, error) {
	root, err := HostRoot()
	if err != nil {
		return "", err
	}
	argv := append([]string{"run", ServicePackage, command}, args...)
	cmd := exec.Command("go", argv...)
	cmd.Dir = root
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go %s (in %s): %w: %s",
			strings.Join(argv, " "), root, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
