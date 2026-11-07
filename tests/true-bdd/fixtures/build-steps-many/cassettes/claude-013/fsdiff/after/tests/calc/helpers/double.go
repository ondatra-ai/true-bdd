package helpers

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// DoubleCommand is the name of the calculator command the doubling concept
// runs: the CLI document declares `calc double VALUE`.
const DoubleCommand = "double"

// Doubled is what one run of the calculator's double command reported: the
// stdout the program printed and the integer read from it. Nothing here is
// calculated from the step's number; the integer is only what the program
// printed.
type Doubled struct {
	// Stdout is the program's stdout exactly as it was printed.
	Stdout string
	// Value is the integer the program printed on stdout.
	Value int
}

// RunDouble runs the kept calculator program's double command as a process
// from the host project root, passing value verbatim as the command's VALUE
// argument, and reports the integer the program printed on stdout. A non-zero
// exit, or stdout that is not an integer, is reported as an error.
func RunDouble(calculator *Calculator, value string) (*Doubled, error) {
	if calculator == nil || calculator.Path == "" {
		return nil, fmt.Errorf("run the calculator's %s command: no calculator is installed; the installation Given must run first", DoubleCommand)
	}
	run := exec.Command(calculator.Path, DoubleCommand, value)
	// The CLI document says the program is run from the host project root.
	run.Dir = calculator.Root
	// Inherit the ambient environment rather than creating a separate one.
	run.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	run.Stdout = &stdout
	run.Stderr = &stderr
	if err := run.Run(); err != nil {
		if reported := strings.TrimSpace(stderr.String()); reported != "" {
			return nil, fmt.Errorf("run %s %s %s in %s: %w: %s", CalculatorName, DoubleCommand, value, calculator.Root, err, reported)
		}
		return nil, fmt.Errorf("run %s %s %s in %s: %w", CalculatorName, DoubleCommand, value, calculator.Root, err)
	}
	printed := stdout.String()
	number, err := strconv.Atoi(strings.TrimSpace(printed))
	if err != nil {
		return nil, fmt.Errorf("read the integer %s %s %s printed on stdout: %q is not an integer", CalculatorName, DoubleCommand, value, printed)
	}
	return &Doubled{Stdout: printed, Value: number}, nil
}
