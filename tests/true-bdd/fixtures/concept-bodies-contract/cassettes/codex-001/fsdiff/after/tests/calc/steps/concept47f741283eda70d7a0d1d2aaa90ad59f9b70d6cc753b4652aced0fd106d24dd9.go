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
	// The suite runs with tests/calc as its working directory, and the halving step
	// runs the tool from the host root, so the stand-in must be an absolute path.
	standIn, err := filepath.Abs(filepath.Join("fixtures", name, "bin", "go"))
	if err != nil {
		return err
	}
	info, err := os.Stat(standIn)
	if err != nil {
		return fmt.Errorf("toolchain %q: %w", name, err)
	}
	if info.Mode()&0o111 == 0 {
		if err := os.Chmod(standIn, info.Mode()|0o755); err != nil {
			return fmt.Errorf("toolchain %q: %w", name, err)
		}
	}
	// The stand-in defers to the installed go, which it expects in CALC_REAL_GO.
	realGo, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("toolchain %q: no installed go: %w", name, err)
	}
	if realGo, err = filepath.Abs(realGo); err != nil {
		return err
	}
	// Keep the inherited environment, including the Go build cache and temporary
	// directory, and only name the real go for the stand-in.
	inherited := os.Environ()
	env := make([]string, 0, len(inherited)+1)
	for _, entry := range inherited {
		if strings.HasPrefix(entry, "CALC_REAL_GO=") {
			continue
		}
		env = append(env, entry)
	}
	state.Go = standIn
	state.Env = append(env, "CALC_REAL_GO="+realGo)
	return nil
}
