package helpers

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Halve runs the calculator's halve command as the Public interface
// (docs/architecture/calc-cli.yaml) declares it:
//
//	go run ./services/calc halve VALUE
//
// from the host root, with value passed verbatim as the single value
// argument, leading minus sign included (no "--" separator). It returns the
// command's integer stdout. The halving itself is the product's work; this
// helper only invokes it and parses what it printed.
func Halve(value string) (int, error) {
	root, err := HostRoot()
	if err != nil {
		return 0, err
	}

	cmd := exec.Command("go", "run", "./services/calc", "halve", value)
	cmd.Dir = root
	// Inherit the ambient environment so the Go build and module caches
	// configured for this run are reused.
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf(
			"go run ./services/calc halve %s (in %s): %w: %s",
			value, root, err, strings.TrimSpace(stderr.String()),
		)
	}

	out := strings.TrimSpace(stdout.String())
	n, err := strconv.Atoi(out)
	if err != nil {
		return 0, fmt.Errorf(
			"go run ./services/calc halve %s: stdout %q is not an integer: %w",
			value, out, err,
		)
	}
	return n, nil
}

// HostRoot returns the host root the CLI commands are documented to run from:
// the nearest ancestor of the test's working directory that holds
// ./services/calc.
func HostRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("locate host root: %w", err)
	}
	start := dir
	for {
		if info, err := os.Stat(filepath.Join(dir, "services", "calc")); err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("host root with ./services/calc not found above %s", start)
		}
		dir = parent
	}
}
