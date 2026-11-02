# Fix Prompt for Item when-halving: when concept "halving"

## Target
tests/calc/steps

## Diagnosis
`tests/calc/steps/concept13b4354efb42d97fb2de7c428a7011995700d0f26a06ebb3f9c6eb1c150d5f87.go`
delegates to the suite helper `State.Halve`, whose first act is to fail unless the
environment variable `CALC_LICENCE` is set. The binding for this concept is only
"Run the declared halve command as a process with the step's whole number and keep
its integer stdout as the result", and the declared Public interface
(`docs/architecture/calc-cli.yaml`) gives `halve` exactly one integer argument and
an integer stdout. No fixture input of `calc-001-halve` or `calc-002-halve` states a
licence or any environment variable, so the step demands more than its binding and
the fixtures state.

The fix is in the step body: run the declared command directly with the step's
whole number and keep its integer stdout, with no precondition the binding and the
interface do not state.

## Required Changes

Replace the whole contents of
`tests/calc/steps/concept13b4354efb42d97fb2de7c428a7011995700d0f26a06ebb3f9c6eb1c150d5f87.go`
with exactly this file. The `//truebdd:concept` record, the function name and the
signature are unchanged; only the body and this file's imports change.

```go
package steps

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "halving"
func concept13b4354efb42d97fb2de7c428a7011995700d0f26a06ebb3f9c6eb1c150d5f87(state *State, args []string) error {
	number := args[0]
	cmd := exec.Command("go", "run", "./services/calc", "halve", number)
	cmd.Dir = "../.."
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("halve %s: %w: %s", number, err, strings.TrimSpace(stderr.String()))
	}
	result, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return fmt.Errorf("halve %s: stdout %q is not an integer: %w", number, string(out), err)
	}
	state.Result = result
	return nil
}
```

Notes on why the body reads this way:

- `args[0]` is the pattern's single capture, the step's whole number as written
  (`6`, `-7`). It is handed to the command verbatim; the body never halves,
  rounds or otherwise computes the product's answer.
- `cmd.Dir = "../.."` is the host root as seen from the `tests/calc` package
  directory, matching the interface document's "Run `go run ./services/calc halve
  VALUE` from the host root".
- stdout is the only observation; it is trimmed and parsed as an integer and kept
  in the existing shared field `State.Result`, which the `then "result"` definition
  already compares against the expected value.
- No environment variable, licence or any other precondition is read: the binding
  and the interface state none.

## Do Not Change

- Do not change `State` in `tests/calc/steps/state.go`: no new or renamed fields are
  needed, since `State.Result` already exists and already carries the integer result
  to the `then` step. `State.Halve` simply stops being used by this concept; leave
  the file, its helper and its imports exactly as they are.
- Do not touch `tests/calc/steps/register.go`, `tests/calc/steps/definitions.go`,
  `tests/calc/scenarios/scenarios.go`, `tests/calc/calc_test.go`, the registry
  `docs/scenarios.yaml`, the Specification or any service code.
- Do not add package-level constants, variables, helper functions or types; every
  value the body needs is local to it.
