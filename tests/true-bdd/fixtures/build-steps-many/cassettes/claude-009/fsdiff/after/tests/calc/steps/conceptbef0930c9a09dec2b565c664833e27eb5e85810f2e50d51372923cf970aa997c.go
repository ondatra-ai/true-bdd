package steps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//truebdd:concept given "installation"
func conceptbef0930c9a09dec2b565c664833e27eb5e85810f2e50d51372923cf970aa997c(state *State, args []string) error {
	hostRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return fmt.Errorf("locate the host root: %w", err)
	}

	buildDir, err := os.MkdirTemp("", "calc-install-")
	if err != nil {
		return fmt.Errorf("make a build directory under the host's tmp directory: %w", err)
	}

	program := filepath.Join(buildDir, "calc")

	build := exec.Command("go", "build", "-o", program, "./services/calc")
	build.Dir = hostRoot
	build.Env = os.Environ()
	if out, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("build the calculator from ./services/calc at %s: %w: %s", hostRoot, err, strings.TrimSpace(string(out)))
	}

	if _, err := os.Stat(program); err != nil {
		return fmt.Errorf("the built calculator program is not at %s: %w", program, err)
	}

	state.CalcPath = program
	return nil
}
