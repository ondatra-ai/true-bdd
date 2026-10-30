package steps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

//truebdd:concept given "installation"
func conceptbef0930c9a09dec2b565c664833e27eb5e85810f2e50d51372923cf970aa997c(state *State, args []string) error {
	program, err := filepath.Abs(filepath.Join("..", "..", "tmp", "calc-program", "calc"))
	if err != nil {
		return fmt.Errorf("resolve calculator program path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(program), 0o755); err != nil {
		return fmt.Errorf("create calculator program directory: %w", err)
	}
	command := exec.Command("go", "build", "-o", program, "./services/calc")
	command.Dir = "../.."
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build calculator: %w: %s", err, output)
	}
	state.Program = program
	return nil
}
