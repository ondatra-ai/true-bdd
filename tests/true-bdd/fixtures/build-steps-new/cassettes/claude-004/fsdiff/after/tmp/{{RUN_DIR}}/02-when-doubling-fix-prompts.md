# Fix Prompt for Item when-doubling: when concept "doubling"

## Target
tests/calc/steps

## Required Changes

Write only the body of the one registered `when "doubling"` definition. No change
to `tests/calc/steps/state.go` (the fields the body needs, `Program` and `Value`,
already exist) and no new files under `tests/calc/helpers/` (a single body shares
nothing). Keep `tests/calc/steps/register.go` untouched.

### 1. Replace the body in `tests/calc/steps/concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b.go`

Keep the `package steps` clause, the `//truebdd:concept when "doubling"` record and
the function name/signature exactly as generated; replace only the import block and
the statements inside the function with the following file content:

```go
package steps

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "doubling"
func concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b(state *State, args []string) error {
	value := args[0]
	if _, err := strconv.Atoi(value); err != nil {
		return fmt.Errorf("double %q: not a whole number: %w", value, err)
	}
	if state.Program == "" {
		return fmt.Errorf("double %s: the calculator is not installed", value)
	}
	command := exec.Command(state.Program, "double", value)
	command.Dir = "../.."
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("run calc double %s: %w: %s", value, err, strings.TrimSpace(stderr.String()))
	}
	doubled, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	if err != nil {
		return fmt.Errorf("calc double %s printed %q: %w", value, stdout.String(), err)
	}
	state.Value = doubled
	return nil
}
```

### Why this body

- The binding says: run the kept calculator program's `double` command as a process
  from the host root with the step's whole number and keep its integer stdout. The
  body therefore takes the program path the Given kept in `State.Program`
  (already absolute, produced by `filepath.Abs` in `installed`), runs it with
  `command.Dir = "../.."` so the process starts from the host root exactly as the
  declared interface describes, and passes the capture through verbatim as the
  single `VALUE` argument.
- `args[0]` is the only thing the body knows of the step: the optionally negative
  integer literal the Owner doubles. It is validated as an integer purely to fail
  the step with a clear message instead of handing junk to the process, then passed
  on as written.
- The declared `0` response is "Doubled integer on stdout" with a trailing newline,
  so the body trims the newline and parses the integer, storing it unexamined in
  `state.Value`. The Then step (`result`) is what compares it with the expected
  number. The body never computes `value + value` or `2 * value` anywhere — the
  only source of the answer is the process's stdout.
- Non-zero exit (declared `1`: invalid arguments) surfaces as a step error that
  includes the captured value and the program's stderr, so a failing run reports
  the product's own complaint rather than a bare exit status.

### Do not add

- Do not write, create or touch `tmp/calc-calls.log` from this step. That file is
  the boundary evidence that the declared command really ran as a process; a step
  definition that wrote it would fabricate the very observation
  `tests/calc/z_command_boundary_test.go` exists to make. The step's only job is to
  run `state.Program double <value>` from the host root.
- Do not re-build the program here and do not fall back to building it: installation
  belongs to the Given (`installed`), and this body only uses what it kept.
- Do not add fields to `State`; `Program` (set by the Given) and `Value` (read by
  the Then) are sufficient.
