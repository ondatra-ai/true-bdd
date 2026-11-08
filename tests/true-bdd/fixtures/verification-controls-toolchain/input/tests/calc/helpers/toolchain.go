// Package helpers holds the code the calc suite's step definitions share.
package helpers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// fixtureRoot is the suite's fixture directory, relative to the directory the
// suite's tests run in (tests/calc).
const fixtureRoot = "fixtures"

// realGoVar is the variable a fixture's Go stand-in reads to find the
// installed go it defers to.
const realGoVar = "CALC_REAL_GO"

// FixtureToolchain returns the absolute path of the named fixture's Go
// stand-in, tests/calc/fixtures/<name>/bin/go, after checking that the suite
// ships it as an executable file. It reports a named toolchain the suite has
// no fixture for as an error instead of silently falling back to go.
func FixtureToolchain(name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("toolchain name is empty")
	}
	if name != filepath.Base(filepath.Clean(name)) {
		return "", fmt.Errorf("toolchain %q is not a single fixture name", name)
	}
	path, err := filepath.Abs(filepath.Join(fixtureRoot, name, "bin", "go"))
	if err != nil {
		return "", fmt.Errorf("locate the %q toolchain: %w", name, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("the %q toolchain needs the fixture %s: %w", name, path, err)
	}
	if info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("the %q toolchain fixture %s is not an executable stand-in for go", name, path)
	}
	return path, nil
}

// ToolchainEnv returns the environment a fixture's Go stand-in runs in: the
// inherited environment, which keeps the Go cache and temporary directory
// settings, with CALC_REAL_GO naming the installed go the stand-in defers to.
func ToolchainEnv() ([]string, error) {
	installed, err := InstalledGo()
	if err != nil {
		return nil, err
	}
	inherited := os.Environ()
	env := make([]string, 0, len(inherited)+1)
	for _, entry := range inherited {
		if strings.HasPrefix(entry, realGoVar+"=") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, realGoVar+"="+installed), nil
}

// InstalledGo returns the absolute path of the installed go, the toolchain a
// fixture's stand-in runs in turn.
func InstalledGo() (string, error) {
	if goroot := os.Getenv("GOROOT"); goroot != "" {
		candidate := filepath.Join(goroot, "bin", "go")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	found, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("locate the installed go: %w", err)
	}
	path, err := filepath.Abs(found)
	if err != nil {
		return "", fmt.Errorf("locate the installed go: %w", err)
	}
	return path, nil
}

