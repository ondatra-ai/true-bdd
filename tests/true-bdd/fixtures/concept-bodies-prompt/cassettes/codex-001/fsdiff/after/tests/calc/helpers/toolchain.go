// Package helpers holds the code the calc suite's step definitions share.
package helpers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FixturesDir is the suite's fixture root, relative to the working directory
// the suite's tests run in (tests/calc).
const FixturesDir = "fixtures"

// RealGoEnv is the variable a fixture's Go stand-in reads to find the real Go
// executable it delegates to.
const RealGoEnv = "CALC_REAL_GO"

// ToolchainStandIn returns the absolute path of the named fixture's Go
// stand-in, tests/calc/fixtures/<name>/bin/go, after checking that it is a
// file that can be executed in place of go.
func ToolchainStandIn(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("toolchain name is empty")
	}
	if filepath.Base(name) != name || name == "." || name == ".." {
		return "", fmt.Errorf("toolchain %q is not a single fixture directory name", name)
	}
	path, err := filepath.Abs(filepath.Join(FixturesDir, name, "bin", "go"))
	if err != nil {
		return "", fmt.Errorf("locate the %q toolchain stand-in: %w", name, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("the %q toolchain stand-in: %w", name, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("the %q toolchain stand-in %s is a directory", name, path)
	}
	if info.Mode().Perm()&0o111 == 0 {
		if err := os.Chmod(path, info.Mode().Perm()|0o111); err != nil {
			return "", fmt.Errorf("the %q toolchain stand-in %s is not executable: %w", name, path, err)
		}
	}
	return path, nil
}

// RealGo returns the absolute path of the real Go executable a fixture's
// stand-in delegates to: an explicit RealGoEnv setting first, then the go on
// PATH that is running the suite, then GOROOT/bin/go.
func RealGo() (string, error) {
	if path, ok := executableFile(os.Getenv(RealGoEnv)); ok {
		return path, nil
	}
	if found, err := exec.LookPath("go"); err == nil {
		if path, ok := executableFile(found); ok {
			return path, nil
		}
	}
	if goroot := os.Getenv("GOROOT"); goroot != "" {
		if path, ok := executableFile(filepath.Join(goroot, "bin", "go")); ok {
			return path, nil
		}
	}
	return "", fmt.Errorf("no real go executable found: put go on PATH or set %s to it", RealGoEnv)
}

// EnvWithRealGo returns the suite process's environment with RealGoEnv set to
// realGo, replacing any inherited setting and keeping every other variable so
// the stand-in runs go with the same caches, temporary directory and home as
// the installed toolchain would.
func EnvWithRealGo(realGo string) []string {
	assignment := RealGoEnv + "=" + realGo
	inherited := os.Environ()
	env := make([]string, 0, len(inherited)+1)
	for _, entry := range inherited {
		if strings.HasPrefix(entry, RealGoEnv+"=") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, assignment)
}

// executableFile reports whether path names a file that can be executed, and
// returns it as an absolute path.
func executableFile(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		return "", false
	}
	return abs, true
}
