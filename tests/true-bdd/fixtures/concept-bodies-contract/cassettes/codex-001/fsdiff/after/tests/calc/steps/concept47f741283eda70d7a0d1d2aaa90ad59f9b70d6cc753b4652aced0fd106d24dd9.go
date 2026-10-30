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

	// The named fixture's Go stand-in, run in place of go. Step bodies run
	// from the suite directory (tests/calc), the same assumption the
	// registry path "../../docs/scenarios.yaml" already makes, so the
	// fixture lives at fixtures/<name>/bin/go. Absolute, because the
	// halving step runs the tool from the host root (cmd.Dir = "../..").
	standIn, err := filepath.Abs(filepath.Join("fixtures", name, "bin", "go"))
	if err != nil {
		return fmt.Errorf("the %q toolchain: resolving its Go stand-in: %w", name, err)
	}
	info, err := os.Stat(standIn)
	if err != nil {
		return fmt.Errorf("the %q toolchain: its Go stand-in %s: %w", name, standIn, err)
	}
	if info.IsDir() {
		return fmt.Errorf("the %q toolchain: its Go stand-in %s is a directory", name, standIn)
	}
	if info.Mode().Perm()&0o111 == 0 {
		if err := os.Chmod(standIn, info.Mode().Perm()|0o111); err != nil {
			return fmt.Errorf("the %q toolchain: making its Go stand-in %s runnable: %w", name, standIn, err)
		}
	}

	// The stand-in runs the installed toolchain, which it reads from
	// CALC_REAL_GO. Carry the inherited environment through unchanged so
	// the Go build cache and temporary directory stay as the suite found
	// them, and add only that one name.
	realGo, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("the %q toolchain: locating the installed go for CALC_REAL_GO: %w", name, err)
	}

	state.Go = standIn
	state.Env = append(os.Environ(), "CALC_REAL_GO="+realGo)
	return nil
}
