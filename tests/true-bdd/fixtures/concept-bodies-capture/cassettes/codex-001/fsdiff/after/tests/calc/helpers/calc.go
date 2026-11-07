// Package helpers runs the calculator CLI declared in
// docs/architecture/calc-cli.yaml on behalf of the calc suite's step
// definitions. Nothing here decides an arithmetic answer: the helpers carry a
// step's argument into the process and the process's printed integer back out.
package helpers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CalcPackage is the Go package the CLI document names for the calculator.
const CalcPackage = "./services/calc"

// commandTimeout bounds a single calculator run, its compilation included.
const commandTimeout = 2 * time.Minute

// Halve runs the declared `halve` command as a process for value and returns
// the integer the command writes on stdout. The calculator alone determines
// that integer.
func Halve(value int) (int, error) {
	stdout, err := runCalc("halve", strconv.Itoa(value))
	if err != nil {
		return 0, err
	}
	printed := strings.TrimSpace(stdout)
	halved, err := strconv.Atoi(printed)
	if err != nil {
		return 0, fmt.Errorf("calc halve %d: stdout %q is not an integer: %w", value, printed, err)
	}
	return halved, nil
}

// runCalc runs `go run ./services/calc <args...>` from the host root and
// returns its stdout. A non-zero exit becomes an error that reports the exit
// code and stderr, so a failing step says what the process said.
func runCalc(args ...string) (string, error) {
	root, err := HostRoot()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	argv := append([]string{"run", CalcPackage}, args...)
	cmd := exec.CommandContext(ctx, "go", argv...)
	cmd.Dir = root
	// Inherit the environment so the caller's Go build cache and temporary
	// directory keep being used.
	cmd.Env = os.Environ()
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		called := CalcPackage + " " + strings.Join(args, " ")
		var exit *exec.ExitError
		if errors.As(runErr, &exit) {
			return "", fmt.Errorf("go run %s in %s: exit status %d: %s", called, root, exit.ExitCode(), strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("go run %s in %s: %w: %s", called, root, runErr, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// HostRoot is the nearest directory at or above the working directory that
// holds the calculator package, which is the directory the CLI document runs
// the command from.
func HostRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("host root: %w", err)
	}
	relative := filepath.FromSlash(strings.TrimPrefix(CalcPackage, "./"))
	for {
		if info, statErr := os.Stat(filepath.Join(dir, relative)); statErr == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("host root: no %s directory at or above the working directory", CalcPackage)
		}
		dir = parent
	}
}
