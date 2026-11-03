# Fix Prompt for Item when-halving: when concept "halving"

## Target
tests/calc/steps

## Required Changes

### 1. Correct the registered pattern so the step captures the whole number

The current pattern `^the Owner halves (-?\d)\d*$` captures only the first digit,
so `the Owner halves 64` hands the body `6` instead of `64`. Per the user's
clarification, widen the capture group to the whole number.

File: `tests/calc/steps/register.go`

Replace this exact line:

```go
	suite.Step("^the Owner halves (-?\\d)\\d*$", concept8c83e3012dd37395db8aad8a8fafef5f14ca67ae936908b0e63bff4b60526148)
```

with:

```go
	suite.Step("^the Owner halves (-?\\d+)$", concept8c83e3012dd37395db8aad8a8fafef5f14ca67ae936908b0e63bff4b60526148)
```

Change nothing else in `register.go`: the `result` registration, the function
name it binds and the file's order stay exactly as they are.

### 2. Write the body of the `when` definition

File: `tests/calc/steps/concept8c83e3012dd37395db8aad8a8fafef5f14ca67ae936908b0e63bff4b60526148.go`

Overwrite the whole file with the content below. The `//truebdd:concept when "halving"`
record, the function name and the signature are preserved verbatim; only the
import list and the body change.

```go
package steps

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//truebdd:concept when "halving"
func concept8c83e3012dd37395db8aad8a8fafef5f14ca67ae936908b0e63bff4b60526148(state *State, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("the Owner halves: expected 1 capture, got %d", len(args))
	}
	value := args[0]
	if _, err := strconv.Atoi(value); err != nil {
		return fmt.Errorf("the Owner halves %q: not a whole number: %w", value, err)
	}
	cmd := exec.Command("go", "run", "./services/calc", "halve", value)
	cmd.Dir = "../.."
	stdout, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("halve %s: %w: %s", value, err, strings.TrimSpace(string(exit.Stderr)))
		}
		return fmt.Errorf("halve %s: %w", value, err)
	}
	printed := strings.TrimSpace(string(stdout))
	halved, err := strconv.Atoi(printed)
	if err != nil {
		return fmt.Errorf("halve %s: stdout %q is not an integer: %w", value, printed, err)
	}
	state.Result = halved
	return nil
}
```

## Why this body

- The binding says: *run the declared halve command as a process with the step's
  whole number and keep its integer stdout as the result*. The body does exactly
  that and nothing more.
- The declared Public interface (`docs/architecture/calc-cli.yaml`) states the
  invocation is `go run ./services/calc halve VALUE` from the host root, with the
  halved integer on stdout for exit `0` and invalid arguments on exit `1`. The
  test binary's working directory is `tests/calc`, so `cmd.Dir = "../.."` is the
  host root; the non-zero exit path surfaces the command's stderr as the step's
  error instead of being swallowed.
- The step never halves anything itself: it only passes the captured number to
  the command and parses the integer the command printed. The arithmetic stays
  in the product.
- `strconv.Atoi(value)` before the call is a guard on the step's own capture, not
  a computation of the product's answer.

## Shared state

No change. `tests/calc/steps/state.go` already declares `type State struct{ Result int }`,
and the existing `then` definition in `tests/calc/steps/definitions.go` reads
`state.Result` to compare against the expected value. The `when` body writes that
same field, so the observation side needs no edit.

## Do not touch

- `tests/calc/steps/definitions.go` (the `then "result"` definition)
- `tests/calc/steps/state.go`
- `tests/calc/scenarios/scenarios.go`
- `tests/calc/calc_test.go` (generated)
- The Specification (`docs/scenarios.yaml`, `docs/architecture/*`) and any service code
