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

	// The fixture's Go stand-in, as the Word mapping names it. Scenario tests run
	// in tests/calc, while the halving runs the calculator from the host root, so
	// the stand-in has to be named absolutely.
	standIn, err := filepath.Abs(filepath.Join("fixtures", name, "bin", "go"))
	if err != nil {
		return fmt.Errorf("toolchain %q: %w", name, err)
	}
	info, err := os.Stat(standIn)
	if err != nil {
		return fmt.Errorf("toolchain %q: no Go stand-in at %s: %w", name, standIn, err)
	}
	if info.IsDir() {
		return fmt.Errorf("toolchain %q: %s is a directory, not a Go stand-in", name, standIn)
	}
	if info.Mode().Perm()&0o111 == 0 {
		if err := os.Chmod(standIn, info.Mode().Perm()|0o755); err != nil {
			return fmt.Errorf("toolchain %q: %s is not executable: %w", name, standIn, err)
		}
	}

	// The stand-in runs the installed go, which it reads from CALC_REAL_GO.
	realGo, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("toolchain %q: no installed go to stand in for: %w", name, err)
	}
	if realGo, err = filepath.Abs(realGo); err != nil {
		return fmt.Errorf("toolchain %q: %w", name, err)
	}

	state.Go = standIn
	state.Env = append(os.Environ(), "CALC_REAL_GO="+realGo)
	return nil
}
