package helpers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// HostRoot is relative to the test process working directory, tests/calc.
// It names the host root where the declared calculator is built and run.
const HostRoot = "../.."

// BuildCalculator builds ./services/calc from HostRoot into its tmp directory
// and returns the program path; it does not run the calculator.
func BuildCalculator() (string, error) {
	program, err := filepath.Abs(filepath.Join(HostRoot, "tmp", "calc-program", "calc"))
	if err != nil {
		return "", fmt.Errorf("resolve calculator program path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(program), 0o755); err != nil {
		return "", fmt.Errorf("create calculator program directory: %w", err)
	}

	cmd := exec.Command("go", "build", "-o", program, "./services/calc")
	cmd.Dir = HostRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build calculator: %w: %s", err, output)
	}

	return program, nil
}
