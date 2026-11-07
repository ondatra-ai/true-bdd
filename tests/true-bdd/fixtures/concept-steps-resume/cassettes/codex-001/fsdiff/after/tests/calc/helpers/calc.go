// Package helpers reaches the calculator service through its declared
// public interface: the OpenCLI document docs/architecture/calc-cli.yaml.
// Nothing here computes a calculator answer; the process on the other
// side of exec is the only thing that does arithmetic.
package helpers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// HalveCommand is the command name the halving concept's binding
// references in the declared CLI.
const HalveCommand = "halve"

// calcPackage is the Go package path the declared interface says the
// calculator CLI is run from: `go run ./services/calc <command> ...`.
const calcPackage = "./services/calc"

// hostRootFallback is the host root relative to a suite's working
// directory (tests/<suite>), the same relative base the suite's harness
// already uses for its registry path.
const hostRootFallback = "../.."

// HostRoot returns the directory the calculator CLI is run from: the
// nearest ancestor of the working directory holding a go.mod, which is
// the host root the declared interface names.
func HostRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return hostRootFallback
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return hostRootFallback
		}
		dir = parent
	}
}

// RunCalcInt runs the named calculator command as a process from the
// host root with the given arguments and returns its stdout read as an
// integer. A non-zero exit, or stdout that is not an integer, is
// reported as an error naming the command line that produced it.
func RunCalcInt(command string, args ...string) (int, error) {
	argv := append([]string{"run", calcPackage, command}, args...)
	line := "go " + strings.Join(argv, " ")

	cmd := exec.Command("go", argv...)
	cmd.Dir = HostRoot()
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			return 0, fmt.Errorf("%s: %w: %s", line, err, detail)
		}
		return 0, fmt.Errorf("%s: %w", line, err)
	}

	out := strings.TrimSpace(stdout.String())
	value, err := strconv.Atoi(out)
	if err != nil {
		return 0, fmt.Errorf("%s: stdout %q is not an integer", line, out)
	}
	return value, nil
}

// WholeNumber reads a step capture as the whole number the glossary
// defines: positive, zero or negative.
func WholeNumber(capture string) (int, error) {
	number, err := strconv.Atoi(capture)
	if err != nil {
		return 0, fmt.Errorf("%q is not a whole number", capture)
	}
	return number, nil
}
