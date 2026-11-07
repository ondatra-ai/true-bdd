# Fix Prompt for Item when-halving: when concept "halving"

## Target
tests/calc/steps

## Required Changes

Write the body of the one listed definition. Nothing else changes: no new
helpers package is needed, `State` already carries the field this body writes,
and `tests/calc/steps/definitions.go`, `register.go` and `state.go` stay
untouched.

### 1. `tests/calc/steps/concept9d6e5581dfa811cf4e3a8cc944e187fb33e1289b93c5e58b45730bc9c8c3d3bb.go`

Replace the whole file with the content below. The `//truebdd:concept` record,
the function name and the signature are preserved verbatim; only the import
list and the body change.

```go
package steps

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "halving"
func concept9d6e5581dfa811cf4e3a8cc944e187fb33e1289b93c5e58b45730bc9c8c3d3bb(state *State, args []string) error {
	cmd := exec.Command("go", "run", "./services/calc", "halve", args[0])
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("halve %s: %w: %s", args[0], err, strings.TrimSpace(string(exit.Stderr)))
		}
		return fmt.Errorf("halve %s: %w", args[0], err)
	}
	result, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return fmt.Errorf("halve %s: stdout %q is not an integer: %w", args[0], string(out), err)
	}
	state.Result = result
	return nil
}
```

### Why this body

- **Binding** — "Run the declared halve command as a process with the step's
  whole number and keep its integer stdout as the result." The body runs the
  command as a process and stores stdout; it never halves anything itself.
  `args[0]` is passed verbatim as the `value` argument, so the capture's minus
  sign (`-7`) reaches the command unchanged, exactly as the capture's meaning
  requires.
- **Public interface** (`docs/architecture/calc-cli.yaml`, command `halve`) —
  invocation is `go run ./services/calc halve VALUE` run "from the host root";
  the suite's tests run in `tests/calc`, so `cmd.Dir = "../.."` is the host
  root, matching the sibling `halveNumber` definition in `definitions.go`.
  Response `'0'` is the halved integer on stdout, which is why stdout is
  trimmed and parsed with `strconv.Atoi`; response `'1'` is a non-zero exit,
  which surfaces as the wrapped `exec.ExitError` with its stderr attached.
- **When block** — the body only invokes the operation and records its outcome
  in shared state. It performs no comparison; the existing
  `^the result is (-?\d+)$` Then definition reads `state.Result` and does the
  comparison for `calc-002-halve` (`the Owner halves -7` / `the result is -3`).
  The expected value `-3` is never computed here.
- **Shared state** — `State.Result int` in `tests/calc/steps/state.go` already
  exists and is the field the Then definition reads, so state is reused as is
  and requires no change.
- **Helpers** — no `tests/calc/helpers/` package is added. This body is the
  only consumer of the invocation code within the bodies this item may write
  (`definitions.go` holds other definitions and must not be edited), so there
  is nothing shared to factor out.
=== FILE_END ===
