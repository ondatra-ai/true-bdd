// Package helpers holds the code the calc suite's step definitions share.
package helpers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// CalculatorPackage is the Go package, relative to the host project root,
// that the declared calculator command is built from. The calculator CLI
// document states the program is built with go build from ./services/calc
// at the host root.
const CalculatorPackage = "./services/calc"

// CalculatorName is the file name given to the built calculator program.
const CalculatorName = "calc"

// BuildDir is the directory, relative to the host project root, the
// calculator program is built into: the host's tmp directory.
const BuildDir = "tmp"

// Calculator is an installed calculator program: the path of the program
// the Given built, and the host project root the CLI document says the
// program is run from.
type Calculator struct {
	// Path is the absolute path of the built calculator program.
	Path string
	// Root is the absolute path of the host project root.
	Root string
}

// HostRoot reports the host project root: the nearest directory at or above
// the working directory that holds a go.mod. The root is located from that
// module file alone, so a host project copied without its services still
// resolves to its own root instead of some directory outside the module.
func HostRoot() (string, error) {
	start, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("locate the working directory: %w", err)
	}
	dir := start
	for {
		info, err := os.Stat(filepath.Join(dir, "go.mod"))
		if err == nil && !info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("locate the host project root: no go.mod at or above %s", start)
		}
		dir = parent
	}
}

// BuildCalculator builds the declared calculator command as a program under
// the host project's tmp directory and reports where it was built, so a When
// can run it from the host project root.
func BuildCalculator() (*Calculator, error) {
	root, err := HostRoot()
	if err != nil {
		return nil, err
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		return nil, fmt.Errorf("find the go tool needed to build the calculator: %w", err)
	}
	outDir := filepath.Join(root, BuildDir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("create the build directory %s: %w", outDir, err)
	}
	program := filepath.Join(outDir, CalculatorName+exeSuffix())

	build := exec.Command(goTool, "build", "-o", program, CalculatorPackage)
	build.Dir = root
	// Inherit the ambient build cache and temporary directory rather than
	// creating a separate one.
	build.Env = os.Environ()
	if output, err := build.CombinedOutput(); err != nil {
		if reported := strings.TrimSpace(string(output)); reported != "" {
			return nil, fmt.Errorf("build %s in %s: %w: %s", CalculatorPackage, root, err, reported)
		}
		return nil, fmt.Errorf("build %s in %s: %w", CalculatorPackage, root, err)
	}

	info, err := os.Stat(program)
	if err != nil {
		return nil, fmt.Errorf("the built calculator program is missing from %s: %w", program, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("the built calculator program at %s is a directory", program)
	}
	return &Calculator{Path: program, Root: root}, nil
}

// exeSuffix is the extension an executable file carries on this platform.
func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
