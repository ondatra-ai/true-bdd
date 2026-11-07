# Fix Prompt for Item when-doubling: when concept "doubling"

## Target
tests/calc/steps

## Finding addressed
`concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b`:
`concept_binding: does not call the bound double operation`.

The function body is the engine's empty skeleton (`return nil`), so the step
never runs the bound operation. The binding requires this When to *run the kept
calculator program's `double` command as a process from the host root with the
step's whole number and keep its integer stdout*. Give the body exactly that
behaviour.

## Required Changes

### 1. Replace the body of the `when` definition

Rewrite `tests/calc/steps/concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b.go`
in full with the content below. Keep the `package steps` clause, the
`//truebdd:concept when "doubling"` record, the function name and the
`func(state *State, args []string) error` signature exactly as they are; only
the import block and the body change.

```go
package steps

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "doubling"
func concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b(state *State, args []string) error {
	if state.Program == "" {
		return errors.New("the calculator is not installed")
	}
	command := exec.Command(state.Program, "double", args[0])
	command.Dir = "../.."
	stdout, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("run calc double %s: %w: %s", args[0], err, exit.Stderr)
		}
		return fmt.Errorf("run calc double %s: %w", args[0], err)
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil {
		return fmt.Errorf("calc double %s printed %q: %w", args[0], stdout, err)
	}
	state.Value = value
	return nil
}
```

### 2. Why each part is required by the binding and the declared interface

- `state.Program` is the calculator kept by the `given "installation"`
  definition in `tests/calc/steps/register.go`, which builds
  `./services/calc` to an absolute path and assigns it. The When consumes that
  kept program rather than building or locating one itself; the empty-string
  guard turns a missing Given into a clear step error instead of an exec
  failure.
- `exec.Command(state.Program, "double", args[0])` is the one call the binding
  asks for: the `double` command of the declared CLI
  (`docs/architecture/calc-cli.yaml`), with the step's captured whole number as
  the required `value` argument, `in: argument`. `args[0]` is passed through
  verbatim — the capture `^the Owner doubles (-?\d+)$` already yields the exact
  integer text the CLI expects, so no reformatting is needed and negative
  operands pass through unchanged.
- `command.Dir = "../.."` runs the process from the host root, as the binding
  states. Steps execute with the working directory at `tests/calc` (the same
  assumption the existing definitions encode: `../../docs/scenarios.yaml` in
  `tests/calc/scenarios/scenarios.go` and `command.Dir = "../.."` in
  `installed`), so `../..` is the host root.
- `command.Output()` captures stdout only, which is where the interface says
  the doubled integer is printed. A non-zero exit (the interface's `'1'`
  response) surfaces as a step error carrying the program's stderr, so a broken
  target fails loudly instead of silently leaving `state.Value` stale.
- `strconv.Atoi(strings.TrimSpace(...))` keeps the integer stdout, trimming the
  trailing newline the interface documents. A non-integer stdout is reported
  with the raw output rather than being coerced to a number.
- `state.Value = value` keeps the observed result for the
  `then "result"` definition, which compares `state.Value` against its own
  expected capture. The When only records what the program printed; it never
  derives the doubled number itself.

### 3. No other changes

- **Do not** add or change any `State` field: `State.Program` (set by the
  Given) and `State.Value` (read by the Then) in `tests/calc/steps/state.go`
  already carry everything this body needs.
- **Do not** create `tests/calc/helpers/`. This is the only body being written
  and it shares no code with another definition, so there is nothing to put
  there; adding a package would be dead weight.
- **Do not** edit `tests/calc/steps/register.go` (registration, `installed`,
  `result`), `tests/calc/scenarios/scenarios.go`, the generated
  `tests/calc/calc_test.go`, `tests/calc/z_command_boundary_test.go`, the
  Specification, or anything under `services/`.
- **Do not** compute `2 * number` anywhere in the steps. The doubled value must
  come only from the program's stdout; the expected values (`6`, `8`) live in
  the Scenarios' `Then` steps and are checked by the existing `result`
  definition.
