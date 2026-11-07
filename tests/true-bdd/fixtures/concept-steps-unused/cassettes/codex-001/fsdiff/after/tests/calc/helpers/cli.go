// Package helpers holds the code the calc suite's step definitions share.
//
// It only drives the calculator's declared CLI and reports what that
// process wrote; it never works out a halved value itself.
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

// HostRoot returns the directory the architecture calls the host root: the
// nearest ancestor of the test's working directory that holds
// docs/architecture/architecture.yaml. The suite runs with its working
// directory at tests/<suite>, so the declared commands, which are stated
// relative to the host root, cannot be run from the working directory.
func HostRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("host root: %w", err)
	}
	for {
		marker := filepath.Join(dir, "docs", "architecture", "architecture.yaml")
		if _, err := os.Stat(marker); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("host root: no ancestor of the working directory holds docs/architecture/architecture.yaml")
		}
		dir = parent
	}
}

// RunHalve runs the declared halve command as a process with value and
// returns the integer the command wrote to stdout. The calculator decides
// the value; RunHalve only reads it back.
func RunHalve(value int) (int, error) {
	root, err := HostRoot()
	if err != nil {
		return 0, err
	}

	cmd := exec.Command("go", "run", "./services/calc", "halve", strconv.Itoa(value))
	cmd.Dir = root
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			return 0, fmt.Errorf("halve %d: %w", value, err)
		}
		return 0, fmt.Errorf("halve %d: %w: %s", value, err, detail)
	}

	text := strings.TrimSpace(stdout.String())
	halved, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("halve %d: stdout %q is not an integer", value, text)
	}
	return halved, nil
}
