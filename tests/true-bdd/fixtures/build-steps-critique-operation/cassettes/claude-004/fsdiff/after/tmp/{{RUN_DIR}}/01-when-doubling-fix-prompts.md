# Fix Prompt for Item when-doubling: when concept "doubling"

## Target
tests/calc/steps

## Required Changes

Edit exactly one file: `tests/calc/steps/concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b.go`.

Replace only the import block and the function body. Keep the `package steps`
clause, the `//truebdd:concept when "doubling"` record and the function name and
signature exactly as they are. Do not touch `tests/calc/steps/register.go`,
`tests/calc/steps/state.go`, `tests/calc/calc_test.go`,
`tests/calc/scenarios/scenarios.go`, the Specification or any service code.

### No shared state change

`State` already carries what this concept needs and must stay as it is:

- `State.Program` — the absolute path of the kept calculator program, set by the
  `given "installation"` step in `register.go`.
- `State.Value` — the integer this `when` step keeps from stdout, read back by
  the `then "result"` step in `register.go`.

Add no field, no package-level constant, variable, helper function or type.

### Write the whole file as

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
	value := args[0]
	command := exec.Command(state.Program, "double", value)
	command.Dir = "../.."
	stdout, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("double %s: %w: %s", value, err, exit.Stderr)
		}
		return fmt.Errorf("double %s: %w", value, err)
	}
	doubled, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil {
		return fmt.Errorf("read the doubled integer of %s from %q: %w", value, stdout, err)
	}
	state.Value = doubled
	return nil
}
```

### Why this body serves the binding

- **Runs the kept program as a process from the host root.** `state.Program` is
  the program the `given` step built with `go build -o ... ./services/calc`;
  `command.Dir = "../.."` is the host root, the same directory the `given` step
  builds from, and matches the interface document's "run from the host root".
  `state.Program` is absolute, so the `Dir` change cannot break the lookup.
- **Passes the step's whole number through unchanged.** `args[0]` is the single
  capture of `^the Owner doubles (-?\d+)$` — an optional minus sign followed by
  digits — handed straight to the command as the required `value` integer
  argument declared by `docs/architecture/calc-cli.yaml`. The step never parses,
  negates or arithmetically touches the operand before the call.
- **Keeps the integer stdout, never computes the product's answer.** The body
  does no doubling of its own: the only integer it stores is the one the
  `double` command printed. `strings.TrimSpace` drops the trailing newline the
  interface document promises on response `'0'`, and `strconv.Atoi` reads the
  printed digits, so `state.Value` holds the program's own result for the
  `then "result"` step to compare.
- **Reports a non-zero exit instead of masking it.** `Output` leaves stdout
  clean for the integer and fills `exec.ExitError.Stderr`, so response `'1'`
  (invalid arguments) surfaces as a step failure with the program's message
  rather than a parse error on empty output.

### Checks

- The function keeps its name, its `(state *State, args []string) error`
  signature and its concept record, so the registration on line 15 of
  `register.go` still resolves.
- `errors`, `fmt`, `os/exec`, `strconv` and `strings` are all standard library,
  so no module requirement changes.
- `fmt` stays used, so the import block has no unused entry.
