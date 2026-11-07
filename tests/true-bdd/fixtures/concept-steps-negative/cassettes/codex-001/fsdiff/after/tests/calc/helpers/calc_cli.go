package helpers

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CommandTimeout bounds a single calculator invocation, so a wedged child
// process fails the step instead of hanging the suite. It is generous because
// the first `go run` in a tree also compiles the service.
const CommandTimeout = 2 * time.Minute

// HostRoot returns the directory the calc bindings call the host root: the
// module root that holds go.mod, from which `go run ./services/calc ...` is
// meant to be invoked. Scenario tests run with their own package directory as
// the working directory, so the root is found by walking upwards.
func HostRoot() (string, error) {
	start, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("locate host root: %w", err)
	}
	dir := start
	for {
		if info, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("locate host root: no go.mod in %s or any parent directory", start)
		}
		dir = parent
	}
}

// Halve runs the calculator's halve command as the halving binding prescribes:
// `go run ./services/calc halve VALUE` from the host root, with value passed
// verbatim as the single VALUE argument. A leading minus sign needs no `--`
// separator because the arguments are handed to the process directly rather
// than through a shell. The command's integer stdout is returned unchanged;
// this helper never halves anything itself.
func Halve(value string) (int, error) {
	stdout, err := RunCalc("halve", value)
	if err != nil {
		return 0, err
	}
	result, err := strconv.Atoi(stdout)
	if err != nil {
		return 0, fmt.Errorf("halve %s: stdout %q is not an integer: %w", value, stdout, err)
	}
	return result, nil
}

// RunCalc invokes the calculator CLI from the host root with the given
// arguments and returns its trimmed stdout. A non-zero exit becomes an error
// that carries stderr, so an absent or failing target reports itself plainly.
func RunCalc(args ...string) (string, error) {
	root, err := HostRoot()
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
	defer cancel()

	argv := append([]string{"run", "./services/calc"}, args...)
	cmd := exec.CommandContext(ctx, "go", argv...)
	cmd.Dir = root

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if ctx.Err() != nil {
			return "", fmt.Errorf("go run ./services/calc %s in %s timed out after %s: %s", strings.Join(args, " "), root, CommandTimeout, detail)
		}
		return "", fmt.Errorf("go run ./services/calc %s in %s: %w: %s", strings.Join(args, " "), root, err, detail)
	}

	return strings.TrimSpace(stdout.String()), nil
}
