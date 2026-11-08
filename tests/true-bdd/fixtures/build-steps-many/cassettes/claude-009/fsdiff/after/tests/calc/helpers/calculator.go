// Package helpers holds the code the calc suite's step definitions share:
// how the calculator program declared by docs/architecture/calc-cli.yaml is
// located and built, and where the built program lives.
package helpers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ServicePackage is the Go package the calculator program is built from.
// The calc CLI declaration states the program is "built with go build from
// ./services/calc at the host root".
const ServicePackage = "./services/calc"

// ProgramName is the name the built calculator program carries on disk. The
// declaration runs it as "calc double VALUE", so the program is calc and
// double is the command it is given.
const ProgramName = "calc"

// BuildDirName is the directory under the Host project root the program is
// built into: the host's tmp directory, as the installation concept binds.
const BuildDirName = "tmp"

// HostRoot returns the root of the Host project the suite runs against: the
// directory of the nearest enclosing go.mod at or above the test's working
// directory. The root is decided by go.mod alone and never by the presence
// of a service directory.
func HostRoot() (string, error) {
	working, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("locate host root: working directory: %w", err)
	}
	dir, err := filepath.Abs(working)
	if err != nil {
		return "", fmt.Errorf("locate host root: absolute path of %s: %w", working, err)
	}
	for {
		info, statErr := os.Stat(filepath.Join(dir, "go.mod"))
		if statErr == nil && !info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("locate host root: no go.mod at or above %s", working)
		}
		dir = parent
	}
}

// ProgramPath is where the calculator program is built for a Host project
// root: ProgramName under the root's tmp directory, with the platform's
// executable suffix.
func ProgramPath(hostRoot string) string {
	name := ProgramName
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(hostRoot, BuildDirName, name)
}

// BuildCalculator builds the calculator program from its declared source
// package into the Host project's tmp directory and returns the path of the
// built program, ready for the When to run.
//
// The build runs from the host root and inherits the test process's
// environment unchanged, so the Go build cache and temporary directory the
// test run was given are the ones used.
func BuildCalculator() (string, error) {
	hostRoot, err := HostRoot()
	if err != nil {
		return "", err
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("build calculator: find the go tool on PATH: %w", err)
	}
	program := ProgramPath(hostRoot)
	buildDir := filepath.Dir(program)
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return "", fmt.Errorf("build calculator: prepare %s: %w", buildDir, err)
	}
	build := exec.Command(goTool, "build", "-o", program, ServicePackage)
	build.Dir = hostRoot
	build.Env = os.Environ()
	output, err := build.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf(
			"build calculator: go build -o %s %s in %s: %w: %s",
			program, ServicePackage, hostRoot, err, strings.TrimSpace(string(output)),
		)
	}
	info, err := os.Stat(program)
	if err != nil {
		return "", fmt.Errorf("build calculator: built program %s: %w", program, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("build calculator: %s is a directory, not a program", program)
	}
	return program, nil
}
