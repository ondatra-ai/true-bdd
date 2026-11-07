// Package helpers holds the code the calc suite's step definitions share.
package helpers

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// DoubleCommand is the declared subcommand of the calculator program that
// doubles its VALUE argument: docs/architecture/calc-cli.yaml states
// "calc double VALUE prints the doubled integer and a newline".
const DoubleCommand = "double"

// CalcSourcePath is the package the declared calculator command is built from,
// relative to the host project root. docs/architecture/calc-cli.yaml states
// the program is "built with go build from ./services/calc at the host root".
const CalcSourcePath = "./services/calc"

// CalcProgramName is the file name given to the built calculator program.
const CalcProgramName = "calc"

// HostRoot returns the host project root: the nearest ancestor of the working
// directory that holds a go.mod file.
func HostRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("working directory: %w", err)
	}
	for {
		info, statErr := os.Stat(filepath.Join(dir, "go.mod"))
		if statErr == nil && !info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found at or above the working directory")
		}
		dir = parent
	}
}

// BuildCalculator builds the declared calculator command as a program under
// the host project's tmp directory and returns the program's path.
func BuildCalculator() (string, error) {
	root, err := HostRoot()
	if err != nil {
		return "", err
	}

	outDir := filepath.Join(root, "tmp")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("create build directory %s: %w", outDir, err)
	}

	name := CalcProgramName
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	program := filepath.Join(outDir, name)

	cmd := exec.Command("go", "build", "-o", program, CalcSourcePath)
	cmd.Dir = root
	// cmd.Env stays unset on purpose: the build inherits the Go build cache
	// and temporary directory of the test run.
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go build %s in %s: %w: %s",
			CalcSourcePath, root, err, strings.TrimSpace(string(out)))
	}

	info, err := os.Stat(program)
	if err != nil {
		return "", fmt.Errorf("built program %s: %w", program, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("built program %s is a directory", program)
	}
	return program, nil
}

// RunDouble runs the kept calculator program's double command as a process
// from the host project root, with value as its VALUE argument, and returns
// the integer the program printed on stdout. It only starts the declared
// command and reads back what the program printed; it never doubles anything
// itself.
func RunDouble(program, value string) (int, error) {
	if strings.TrimSpace(program) == "" {
		return 0, fmt.Errorf("no calculator program kept: the installation Given must run first")
	}

	root, err := HostRoot()
	if err != nil {
		return 0, err
	}

	cmd := exec.Command(program, DoubleCommand, value)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("%s %s %s in %s: %w: %s",
			program, DoubleCommand, value, root, err, strings.TrimSpace(stderr.String()))
	}

	printed := strings.TrimSpace(stdout.String())
	result, err := strconv.Atoi(printed)
	if err != nil {
		return 0, fmt.Errorf("%s %s %s printed %q on stdout, want an integer",
			program, DoubleCommand, value, printed)
	}
	return result, nil
}

// ExpectedNumber reads the whole number a Then step wrote, as the step's
// capture hands it over: an optionally negative integer in decimal. It only
// reads the step's own text and never doubles or otherwise computes anything.
func ExpectedNumber(capture string) (int, error) {
	expected, err := strconv.Atoi(strings.TrimSpace(capture))
	if err != nil {
		return 0, fmt.Errorf("the result step expects a whole number, got %q", capture)
	}
	return expected, nil
}
