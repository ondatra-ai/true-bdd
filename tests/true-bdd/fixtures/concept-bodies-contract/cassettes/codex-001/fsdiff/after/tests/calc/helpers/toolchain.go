// Package helpers holds the code the calc suite's step definitions share.
package helpers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RealGoEnv is the variable a fixture's Go stand-in requires: it names the
// real Go executable the stand-in runs.
const RealGoEnv = "CALC_REAL_GO"

// fixtureRoots are the places the suite's fixture directory can sit relative
// to a test binary's working directory: the suite directory itself first,
// then the host root, then a sibling package's directory.
var fixtureRoots = []string{
	"fixtures",
	filepath.Join("tests", "calc", "fixtures"),
	filepath.Join("..", "fixtures"),
}

// ToolchainGo returns the absolute path of the named fixture's Go stand-in,
// tests/calc/fixtures/<name>/bin/go. The path is absolute because a caller
// runs it with a working directory of its own.
func ToolchainGo(name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("toolchain name is empty")
	}
	if name != filepath.Base(filepath.Clean(name)) {
		return "", fmt.Errorf("toolchain %q is not a fixture directory name", name)
	}
	tried := make([]string, 0, len(fixtureRoots))
	for _, root := range fixtureRoots {
		abs, err := filepath.Abs(filepath.Join(root, name, "bin", "go"))
		if err != nil {
			return "", err
		}
		info, err := os.Stat(abs)
		if err != nil || info.IsDir() {
			tried = append(tried, abs)
			continue
		}
		if err := ensureExecutable(abs, info); err != nil {
			return "", err
		}
		return abs, nil
	}
	return "", fmt.Errorf("toolchain %q has no Go stand-in at bin/go; looked for %s", name, strings.Join(tried, ", "))
}

// ensureExecutable gives a checked-out stand-in its executable bit back, so a
// checkout that dropped file modes still selects the toolchain.
func ensureExecutable(path string, info os.FileInfo) error {
	mode := info.Mode().Perm()
	if mode&0o111 != 0 {
		return nil
	}
	if err := os.Chmod(path, mode|0o111); err != nil {
		return fmt.Errorf("make Go stand-in %s executable: %w", path, err)
	}
	return nil
}

// RealGo returns the path of the installed Go executable, the one a fixture's
// stand-in runs through CALC_REAL_GO. A CALC_REAL_GO already set in the host
// environment wins, so a host that keeps go off PATH can still name it.
func RealGo() (string, error) {
	if named := os.Getenv(RealGoEnv); named != "" {
		if info, err := os.Stat(named); err == nil && !info.IsDir() {
			return named, nil
		}
	}
	if found, err := exec.LookPath("go"); err == nil {
		if abs, err := filepath.Abs(found); err == nil {
			return abs, nil
		}
		return found, nil
	}
	if root := os.Getenv("GOROOT"); root != "" {
		candidate := filepath.Join(root, "bin", "go")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no installed go found on PATH or under GOROOT to name in %s", RealGoEnv)
}

// ToolchainEnv is the host environment with CALC_REAL_GO naming realGo: the
// environment a fixture's Go stand-in requires. Every other variable is
// carried through unchanged, so the inherited build cache and temporary
// directory keep their settings.
func ToolchainEnv(realGo string) []string {
	host := os.Environ()
	env := make([]string, 0, len(host)+1)
	for _, entry := range host {
		if strings.HasPrefix(entry, RealGoEnv+"=") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, RealGoEnv+"="+realGo)
}
