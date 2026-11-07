// Package helpers runs the calculator CLI commands the calc suite's steps
// drive, exactly as the declared interface in docs/architecture/calc-cli.yaml
// states them: `go run ./services/calc <command> ...` from the host root, with
// the command's stdout read back as an integer. Nothing here computes a
// calculator answer; it only starts the process and reports what it printed.
package helpers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// commandPackage is the Go package the declared CLI names for the calculator.
const commandPackage = "./services/calc"

// HostRoot returns the host root the declared CLI is run from: the nearest
// ancestor of the test's working directory that holds the calculator package.
func HostRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("working directory: %w", err)
	}
	for {
		info, statErr := os.Stat(filepath.Join(dir, "services", "calc"))
		if statErr == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("host root holding %s not found above the working directory", commandPackage)
		}
		dir = parent
	}
}

// RunInt runs the named calculator command as a process from the host root with
// the given arguments and returns its stdout as an integer.
func RunInt(command string, args ...string) (int, error) {
	root, err := HostRoot()
	if err != nil {
		return 0, err
	}
	argv := append([]string{"run", commandPackage, command}, args...)
	cmd := exec.Command("go", argv...)
	cmd.Dir = root
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		return 0, fmt.Errorf("go %s in %s: %w: %s",
			strings.Join(argv, " "), root, runErr, strings.TrimSpace(stderr.String()))
	}
	printed := strings.TrimSpace(stdout.String())
	value, convErr := strconv.Atoi(printed)
	if convErr != nil {
		return 0, fmt.Errorf("go %s: stdout %q is not an integer: %w",
			strings.Join(argv, " "), printed, convErr)
	}
	return value, nil
}
