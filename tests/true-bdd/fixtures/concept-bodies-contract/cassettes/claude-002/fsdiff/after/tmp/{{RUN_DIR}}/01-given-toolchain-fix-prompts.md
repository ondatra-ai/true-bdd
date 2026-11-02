# Fix Prompt for Item given-toolchain: given concept "toolchain"

## Target
tests/calc/steps

## Evidence this body rests on

- Binding: "Makes the next halving run the named fixture's Go stand-in,
  `tests/calc/fixtures/<name>/bin/go`, in place of go."
- `tests/calc/steps/halving.go` (the `when` concept, not to be edited) already reads the
  setup this Given must leave behind: it runs `state.Go` (falling back to `"go"`) as
  `<tool> run ./services/calc halve <value>` with `cmd.Dir = "../.."` and
  `cmd.Env = state.Env`. So the Given only has to name the stand-in in `state.Go`
  and provide the environment it needs in `state.Env`.
- `tests/calc/fixtures/offline/bin/go` is a `/bin/sh` stand-in that refuses to run
  (`exit 97`, "CALC_REAL_GO must name the real go") unless `CALC_REAL_GO` names the real
  toolchain, which it then execs with `GOPROXY=off`. Hence the Given must put
  `CALC_REAL_GO` into `state.Env` on top of the inherited environment, which also keeps
  the inherited `GOCACHE`/`TMPDIR`/`HOME` that `go run` needs.
- `tests/calc/scenarios/scenarios.go` loads the registry as `../../docs/scenarios.yaml`,
  and `halving.go` uses `cmd.Dir = "../.."`, so the suite's working directory is
  `tests/calc` and `../..` is the host root the binding's path is written against.
- `State` already has the three fields needed (`Result`, `Go`, `Env`). **No shared state
  change is required.**
- Declared libraries (`docs/architecture/architecture.yaml`) are go-test plus the
  true-bdd pkg module only, so the body uses the standard library.

## Required Changes

Replace **only the body** of the existing function in
`tests/calc/steps/concept47f741283eda70d7a0d1d2aaa90ad59f9b70d6cc753b4652aced0fd106d24dd9.go`,
and widen that file's import block. Keep the `//truebdd:concept given "toolchain"`
record, the function name and the signature exactly as they are. Do not touch
`register.go`, `halving.go`, `definitions.go`, `state.go`, the generated test or any
fixture.

Write the file as exactly this:

```go
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
```

## Notes for the applier

- The only outside-the-body change is this file's import block (`fmt` grows to
  `fmt`, `os`, `os/exec`, `path/filepath`). No package-level constant, variable, type
  or helper is added, and `State` is unchanged.
- The body performs no halving and asserts nothing: it resolves the named fixture's
  stand-in, proves it is a runnable file, and hands it plus its environment to the
  existing `when` step through `state.Go` and `state.Env`.
- Do not run the scenario to check this; the engine owns compilation and the
  whole-suite checks.
