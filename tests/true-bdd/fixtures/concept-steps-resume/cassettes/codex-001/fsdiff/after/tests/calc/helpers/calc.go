package helpers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ServicePackage is the Go package of the calculator CLI, named relative to
// the host root exactly as docs/architecture/calc-cli.yaml names it.
const ServicePackage = "./services/calc"

// HostRoot walks up from the test's working directory to the directory that
// holds the calculator service package, so the declared command runs "from
// the host root" whatever directory the suite is invoked from.
func HostRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	rel := filepath.FromSlash(strings.TrimPrefix(ServicePackage, "./"))
	for {
		if _, err := os.Stat(filepath.Join(dir, rel)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("host root holding %s not found above the working directory", ServicePackage)
		}
		dir = parent
	}
}

// RunCalc runs the named calculator command as a process from the host root
// and returns its stdout with surrounding whitespace removed. A non-zero exit
// is reported with the command's stderr, which is how the CLI announces
// invalid arguments.
func RunCalc(command string, args ...string) (string, error) {
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
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			return "", fmt.Errorf("go %s: %w", strings.Join(argv, " "), err)
		}
		return "", fmt.Errorf("go %s: %w: %s", strings.Join(argv, " "), err, detail)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// RunCalcInt runs the named calculator command and reads its stdout as the
// integer the CLI documents it prints.
func RunCalcInt(command string, args ...string) (int, error) {
	out, err := RunCalc(command, args...)
	if err != nil {
		return 0, err
	}
	value, err := strconv.Atoi(out)
	if err != nil {
		return 0, fmt.Errorf("stdout of calculator %s is not an integer: %q", command, out)
	}
	return value, nil
}
