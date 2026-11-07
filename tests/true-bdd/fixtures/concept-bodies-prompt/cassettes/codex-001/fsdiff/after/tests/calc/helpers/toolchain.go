// Package helpers holds code the calc suite's step definitions share.
package helpers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// HostRoot is the path from the suite directory, tests/calc, where the step
// definitions run, to the host root a calculator command is run from.
const HostRoot = "../.."

// FixtureRoot names the suite's fixture directory from the host root, as the
// Specification states it: tests/calc/fixtures.
const FixtureRoot = "tests/calc/fixtures"

// RealGoEnv is the variable a fixture's Go stand-in reads to find the real Go
// executable it runs in turn.
const RealGoEnv = "CALC_REAL_GO"

// ToolchainStandIn returns the absolute path of the named fixture's Go
// stand-in, tests/calc/fixtures/<name>/bin/go, after checking that something
// runnable is there. The suite directory is tried as well as the host root so
// the lookup holds wherever the suite is run from.
func ToolchainStandIn(name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("toolchain: no fixture name given")
	}
	candidates := []string{
		filepath.Join(HostRoot, FixtureRoot, name, "bin", "go"),
		filepath.Join("fixtures", name, "bin", "go"),
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if info.IsDir() || info.Mode().Perm()&0o111 == 0 {
			return "", fmt.Errorf("toolchain %q: %s is not an executable Go stand-in", name, candidate)
		}
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			return "", fmt.Errorf("toolchain %q: %w", name, err)
		}
		return absolute, nil
	}
	return "", fmt.Errorf("toolchain %q: no Go stand-in at %s", name, strings.Join(candidates, " or "))
}

// RealGo returns the installed Go executable, the one a fixture's stand-in runs
// in turn. A CALC_REAL_GO already in the environment wins, so the real Go can
// be pinned from outside the suite.
func RealGo() (string, error) {
	if pinned := strings.TrimSpace(os.Getenv(RealGoEnv)); pinned != "" {
		return pinned, nil
	}
	if root := strings.TrimSpace(os.Getenv("GOROOT")); root != "" {
		candidate := filepath.Join(root, "bin", "go")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	found, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("find the installed go: %w", err)
	}
	return found, nil
}

// ToolchainEnv returns the environment a calculator command runs with once a
// fixture's Go stand-in is selected: the suite's own environment, so the
// inherited Go build and module caches stay in place, with CALC_REAL_GO naming
// the real Go the stand-in runs.
func ToolchainEnv(realGo string) []string {
	return append(os.Environ(), RealGoEnv+"="+realGo)
}
