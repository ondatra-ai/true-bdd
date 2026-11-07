// Package helpers holds the setup and lookups the calc suite's step
// definitions share.
package helpers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// fixturesDir is the suite's fixture root, relative to the directory a calc
// suite test runs in (tests/calc).
const fixturesDir = "fixtures"

// realGoVar is the variable a fixture's stand-in reads to find the real Go
// executable it delegates to.
const realGoVar = "CALC_REAL_GO"

// SelectToolchain selects the named fixture's Go stand-in,
// tests/calc/fixtures/<name>/bin/go, to be run in place of go.
//
// It returns the absolute stand-in to run and the environment it needs: the
// caller's own environment with CALC_REAL_GO naming the real Go executable,
// which the stand-in requires. The path is absolute because the calculator is
// run from the host root rather than from the suite's directory.
//
// SelectToolchain only chooses the toolchain; it runs nothing.
func SelectToolchain(name string) (string, []string, error) {
	standIn, err := standInPath(name)
	if err != nil {
		return "", nil, err
	}
	realGo, err := realGoPath()
	if err != nil {
		return "", nil, fmt.Errorf("the %q toolchain needs the real go: %w", name, err)
	}
	return standIn, append(os.Environ(), realGoVar+"="+realGo), nil
}

// standInPath is the absolute path of the named fixture's Go stand-in, checked
// to be a file that can be run.
func standInPath(name string) (string, error) {
	path, err := filepath.Abs(filepath.Join(fixturesDir, name, "bin", "go"))
	if err != nil {
		return "", fmt.Errorf("locate the %q toolchain: %w", name, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("the %q toolchain's go stand-in: %w", name, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("the %q toolchain's go stand-in %s is not a file", name, path)
	}
	if info.Mode().Perm()&0o111 == 0 {
		if err := os.Chmod(path, info.Mode().Perm()|0o111); err != nil {
			return "", fmt.Errorf("make the %q toolchain's go stand-in %s runnable: %w", name, path, err)
		}
	}
	return path, nil
}

// realGoPath is the absolute path of the installed Go executable, the one a
// fixture's stand-in delegates to.
func realGoPath() (string, error) {
	found, err := exec.LookPath("go")
	if err != nil {
		if root := os.Getenv("GOROOT"); root != "" {
			if candidate := filepath.Join(root, "bin", "go"); fileExists(candidate) {
				return candidate, nil
			}
		}
		return "", fmt.Errorf("find go on PATH: %w", err)
	}
	abs, err := filepath.Abs(found)
	if err != nil {
		return "", fmt.Errorf("resolve go at %s: %w", found, err)
	}
	return abs, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
