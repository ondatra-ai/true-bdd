package steps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

//truebdd:concept given "installation"
func concept08bdc8ba20fbc17f8cbea598ca6da5bdbc121817e637f18277eb83c75896fcf3(state *State, args []string) error {
	// The declared CLI says the calculator is built with go build from
	// ./services/calc at the host root; the binding says the built program
	// lives under the host's tmp directory and its path is kept for the When.
	const (
		servicePackage = "./services/calc"
		programName    = "calc"
		tmpDirName     = "tmp"
		buildPrefix    = "calc-install-"
	)

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("the calculator is installed: working directory: %w", err)
	}

	// The suite runs from its own package directory, so find the host root by
	// walking up to the directory that holds go.mod.
	hostRoot := ""
	for dir := workDir; ; {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			hostRoot = dir
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return fmt.Errorf("the calculator is installed: no host root with go.mod at or above %s", workDir)
		}
		dir = parent
	}

	tmpRoot := filepath.Join(hostRoot, tmpDirName)
	if err := os.MkdirAll(tmpRoot, 0o755); err != nil {
		return fmt.Errorf("the calculator is installed: host tmp directory %s: %w", tmpRoot, err)
	}

	buildDir, err := os.MkdirTemp(tmpRoot, buildPrefix)
	if err != nil {
		return fmt.Errorf("the calculator is installed: build directory under %s: %w", tmpRoot, err)
	}

	program := filepath.Join(buildDir, programName)
	build := exec.Command("go", "build", "-o", program, servicePackage)
	build.Dir = hostRoot
	if output, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("the calculator is installed: go build -o %s %s in %s: %w: %s", program, servicePackage, hostRoot, err, output)
	}

	if _, err := os.Stat(program); err != nil {
		return fmt.Errorf("the calculator is installed: built program %s: %w", program, err)
	}

	state.Program = program
	state.HostRoot = hostRoot

	return nil
}
