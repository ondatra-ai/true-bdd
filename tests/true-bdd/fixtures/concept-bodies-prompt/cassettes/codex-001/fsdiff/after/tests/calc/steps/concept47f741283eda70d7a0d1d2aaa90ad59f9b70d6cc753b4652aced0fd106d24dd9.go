package steps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//truebdd:concept given "toolchain"
func concept47f741283eda70d7a0d1d2aaa90ad59f9b70d6cc753b4652aced0fd106d24dd9(state *State, args []string) error {
	name := args[0]
	if name == "" {
		return fmt.Errorf("toolchain name is empty")
	}
	// The suite's fixtures live beside the scenario package; the test's working
	// directory is tests/calc, while the halve command is run from the host
	// root, so the stand-in has to be named absolutely.
	standIn, err := filepath.Abs(filepath.Join("fixtures", name, "bin", "go"))
	if err != nil {
		return fmt.Errorf("%q toolchain: %w", name, err)
	}
	info, err := os.Stat(standIn)
	if err != nil {
		return fmt.Errorf("%q toolchain: %w", name, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%q toolchain: %s is a directory", name, standIn)
	}
	if info.Mode().Perm()&0o111 == 0 {
		// A materialised fixture may arrive without its executable bit; the
		// stand-in is a script meant to be run in place of go.
		if err := os.Chmod(standIn, info.Mode().Perm()|0o755); err != nil {
			return fmt.Errorf("%q toolchain: %w", name, err)
		}
	}
	// The stand-in delegates to the installed go, which it expects to be named
	// by CALC_REAL_GO.
	realGo, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("%q toolchain: %w", name, err)
	}
	if realGo, err = filepath.Abs(realGo); err != nil {
		return fmt.Errorf("%q toolchain: %w", name, err)
	}
	env := []string{}
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "CALC_REAL_GO=") {
			continue
		}
		env = append(env, entry)
	}
	state.Env = append(env, "CALC_REAL_GO="+realGo)
	state.Go = standIn
	return nil
}
