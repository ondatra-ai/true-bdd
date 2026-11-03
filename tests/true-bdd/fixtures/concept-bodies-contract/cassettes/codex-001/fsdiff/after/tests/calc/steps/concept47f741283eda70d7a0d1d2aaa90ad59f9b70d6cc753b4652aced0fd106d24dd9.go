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
	if name == "" || name != filepath.Base(name) {
		return fmt.Errorf("toolchain %q is not a fixture directory name", name)
	}
	// The suite runs from tests/calc, the same anchor the halving step's
	// working directory uses, so "../.." is the host root the fixture path
	// in the concept's binding is written against.
	hostRoot, err := filepath.Abs("../..")
	if err != nil {
		return fmt.Errorf("host root: %w", err)
	}
	tool := filepath.Join(hostRoot, "tests", "calc", "fixtures", name, "bin", "go")
	info, err := os.Stat(tool)
	if err != nil {
		return fmt.Errorf("%q toolchain: %w", name, err)
	}
	if info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("%q toolchain %s is not an executable file", name, tool)
	}
	// The fixture's stand-in runs the installed go it is handed, so name it
	// absolutely: the halved command runs from the host root, not from here.
	realGo := os.Getenv("CALC_REAL_GO")
	if realGo == "" {
		if realGo, err = exec.LookPath("go"); err != nil {
			return fmt.Errorf("%q toolchain needs the installed go: %w", name, err)
		}
	}
	if realGo, err = filepath.Abs(realGo); err != nil {
		return fmt.Errorf("%q toolchain needs the installed go: %w", name, err)
	}
	state.Go = tool
	// Inherit this process's environment, which carries the Go build cache and
	// temporary directory, and add the real go for the stand-in to run. A later
	// duplicate key wins in exec.Cmd.Env, so appending is enough.
	state.Env = append(os.Environ(), "CALC_REAL_GO="+realGo)
	return nil
}
