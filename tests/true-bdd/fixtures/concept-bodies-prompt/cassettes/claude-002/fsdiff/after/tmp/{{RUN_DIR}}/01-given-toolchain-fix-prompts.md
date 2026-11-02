# Fix Prompt for Item given-toolchain: given concept "toolchain"

## Target
tests/calc/steps

## Required Changes

Edit only `tests/calc/steps/concept47f741283eda70d7a0d1d2aaa90ad59f9b70d6cc753b4652aced0fd106d24dd9.go`:
replace the placeholder body and widen the import block. Keep the
`//truebdd:concept given "toolchain"` record, the function name and the signature
exactly as they are. No other file changes: `State` already carries `Go` and `Env`,
which the `when` step `halving` reads, so no shared-state change is needed.

Write the file as:

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
```

## Why this body

- Binding: "Makes the next halving run the named fixture's Go stand-in,
  `tests/calc/fixtures/<name>/bin/go`, in place of go." The body only records that
  stand-in in `state.Go`; the existing `when` step (`halving`) is what invokes the
  declared `halve` command with it, so no halving and no arithmetic happens here.
- The absolute path is built from the host root the way `halving` addresses it
  (`cmd.Dir = "../.."`, matching `scenarios.New`'s `../../docs/scenarios.yaml`), so the
  path handed to `exec.Command` resolves the same file regardless of the child's
  working directory.
- `os.Stat` plus the executable-bit check fail the Given loudly when the named
  toolchain is not a fixture of this suite, instead of deferring a confusing
  failure to the `when` step.
- `state.Env` must be populated because `halving` assigns `cmd.Env = state.Env`
  verbatim: a nil value would hand the stand-in an empty environment. The parent
  environment is passed through, plus `CALC_REAL_GO`, which
  `tests/calc/fixtures/offline/bin/go` requires to name the real go (it exits 97
  otherwise). The installed go is located with `exec.LookPath("go")`, matching the
  glossary's "a named fixture's stand-in or the installed one".

## Constraints respected

- No package-level constants, variables, helpers or types added; every literal lives
  inside the function body, and only this file's import block changes outside it.
- `tests/calc/steps/register.go`, `halving.go`, `definitions.go`, `state.go`,
  the generated `tests/calc/calc_test.go`, `docs/scenarios.yaml` and service code are
  left untouched.
