package helpers

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// HostRoot is the host root, written relative to the directory the calc suite's
// tests run in (tests/calc). It is the same directory the kept calculator
// program was built from, and the directory the program is run from.
const HostRoot = "../.."

// Double runs the kept calculator program's double command as a process from the
// host root, with value as its single `value` argument exactly as the step wrote
// it, and returns the integer the process printed on stdout.
//
// Double never doubles anything itself: the number it returns is only ever the
// number the calculator program printed. program is the path to the program kept
// by the installation step.
func Double(program string, value string) (int, error) {
	if program == "" {
		return 0, fmt.Errorf("double %s: no calculator program was kept by the installation step", value)
	}
	command := exec.Command(program, "double", value)
	command.Dir = HostRoot
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return 0, fmt.Errorf("run %s double %s: %w: %s", program, value, err, strings.TrimSpace(stderr.String()))
	}
	printed := strings.TrimSpace(stdout.String())
	doubled, err := strconv.Atoi(printed)
	if err != nil {
		return 0, fmt.Errorf("read the double command's stdout %q as an integer: %w", printed, err)
	}
	return doubled, nil
}
