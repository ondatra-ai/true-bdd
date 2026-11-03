package steps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

//truebdd:concept given "toolchain"
func concept47f741283eda70d7a0d1d2aaa90ad59f9b70d6cc753b4652aced0fd106d24dd9(state *State, args []string) error {
	name := args[0]
	// The halving step runs the calculator's halve command from the host root,
	// two levels up from this suite's working directory (tests/calc).
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return fmt.Errorf("toolchain %q: host root: %w", name, err)
	}
	stand := filepath.Join(root, "tests", "calc", "fixtures", name, "bin", "go")
	info, err := os.Stat(stand)
	if err != nil {
		return fmt.Errorf("toolchain %q: %w", name, err)
	}
	if info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("toolchain %q: %s is not an executable Go stand-in", name, stand)
	}
	installed, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("toolchain %q: installed go: %w", name, err)
	}
	state.Go = stand
	state.Env = append(os.Environ(), "CALC_REAL_GO="+installed)
	return nil
}
