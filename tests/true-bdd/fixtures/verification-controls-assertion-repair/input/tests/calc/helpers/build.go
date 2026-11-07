package helpers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// HostRoot is relative to the test process working directory, tests/calc.
const HostRoot = "../.."

// BuildCalculator builds the declared calculator command (./services/calc)
// from its source with `go build`, run from the host root, placing the
// resulting program at the host's tmp/calc-program/calc. It does not run
// the built program (that is the When step's job); it only returns the
// built program's absolute path so the When step can invoke it regardless
// of working directory.
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

