package steps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//truebdd:concept given "installation"
func concept08bdc8ba20fbc17f8cbea598ca6da5bdbc121817e637f18277eb83c75896fcf3(state *State, args []string) error {
	const (
		hostRootFromSuite = "../.."
		commandPackage    = "./services/calc"
		commandName       = "double"
		programName       = "calc"
		buildDirPattern   = "calc-install-"
	)

	if state.CalcBinary != "" {
		return nil
	}

	hostRoot, err := filepath.Abs(hostRootFromSuite)
	if err != nil {
		return fmt.Errorf("resolve the host root %q: %w", hostRootFromSuite, err)
	}

	buildDir, err := os.MkdirTemp("", buildDirPattern)
	if err != nil {
		return fmt.Errorf("create a build directory under the host's tmp directory: %w", err)
	}

	program := filepath.Join(buildDir, programName)

	build := exec.Command("go", "build", "-o", program, commandPackage)
	build.Dir = hostRoot
	if output, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"build the %q command from %s at the host root %s: %w: %s",
			commandName, commandPackage, hostRoot, err, strings.TrimSpace(string(output)),
		)
	}

	state.HostRoot = hostRoot
	state.CalcBinary = program

	return nil
}
