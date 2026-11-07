# Fix Prompt for Item when-halving: when concept "halving"

## Target
tests/calc/steps

## Required Changes

Two files change. The engine's registration in `tests/calc/steps/register.go`,
the pattern `^the Owner halves (-?\d+)$`, the function name/signature and the
`//truebdd:concept when "halving"` record are all preserved untouched.

### 1. Write the body — `tests/calc/steps/concept13b4354efb42d97fb2de7c428a7011995700d0f26a06ebb3f9c6eb1c150d5f87.go`

The binding for `when "halving"` is: *"Run the declared halve command as a
process with the step's whole number and keep its integer stdout as the
result."* The suite already owns that runner as the shared `State.Halve`
method, so the When body only hands its one capture — the whole number — to
it. It performs no arithmetic itself: the halving is done by the `halve`
process, and the comparison belongs to the `then "result"` definition.

Replace the whole file with exactly:

```go
package steps

import "fmt"

//truebdd:concept when "halving"
func concept13b4354efb42d97fb2de7c428a7011995700d0f26a06ebb3f9c6eb1c150d5f87(state *State, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("the Owner halves: expected 1 capture, got %d", len(args))
	}
	return state.Halve(args[0])
}
```

`args[0]` is the step's whole number (`"6"` for `the Owner halves 6`, `"-7"`
for `the Owner halves -7`) and is passed through verbatim as the command's
`value` argument, which the declared CLI types as an integer.

### 2. Correct the shared runner — `tests/calc/steps/state.go`

`State.Halve` currently refuses to run unless a `CALC_LICENCE` environment
variable is set:

```go
	if os.Getenv("CALC_LICENCE") == "" {
		return errors.New("the calculator's licence is not installed: set CALC_LICENCE")
	}
```

That precondition is not in the Specification. The declared Public interface
`docs/architecture/calc-cli.yaml` describes `halve` as `go run ./services/calc
halve VALUE` from the host root with a single required `value` argument,
exit `0` for the halved integer on stdout and exit `1` for invalid arguments
only — no licence, no environment input. The concept binding, the `number`
glossary entry and the Word mapping likewise mention only the integer argument
and stdout, and `CALC_LICENCE` occurs nowhere else in the suite or the docs.
A gate the declaration does not state would make the When step fail for a
reason unrelated to the product's behaviour, so remove it and let the step
observe the declared command directly.

Replace the whole file with exactly:

```go
package steps

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type State struct{ Result int }

type Registrar interface {
	Step(string, func(*State, []string) error)
}

// Halve runs the calculator's halve command as a process with the whole
// number and keeps its integer stdout as the result. The declared CLI takes
// the value as its only argument and prints the halved integer on stdout, so
// this helper only passes the number through and reads the number back.
func (s *State) Halve(value string) error {
	cmd := exec.Command("go", "run", "./services/calc", "halve", value)
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) > 0 {
			return fmt.Errorf("halve %s: %w: %s", value, err, strings.TrimSpace(string(exit.Stderr)))
		}
		return fmt.Errorf("halve %s: %w", value, err)
	}
	result, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return fmt.Errorf("halve %s: stdout %q is not an integer: %w", value, string(out), err)
	}
	s.Result = result
	return nil
}
```

Changes from the current file, and nothing else:

- Dropped the `CALC_LICENCE` gate, which the declared interface does not
  require, and with it the now-unused `os` import.
- Kept the command, its `value` argument and `cmd.Dir = "../.."` exactly as
  they were: the test package runs in `tests/calc`, so `../..` is the host
  root the interface document names.
- Kept the integer stdout as `s.Result`, which the `then "result"` definition
  in `definitions.go` already compares against the step's expected number.
- Added the command name, the value and any stderr to the returned errors so a
  failing run reports what the process did. This only reports; it never
  computes or substitutes a halved value, and on any error `s.Result` is left
  alone rather than being given a made-up number.

### Not changed

- No `tests/calc/helpers/` package is added: the code these steps share is the
  single `State.Halve` runner that the suite already owns on its shared State,
  and the task is to reuse it rather than duplicate it elsewhere.
- `tests/calc/steps/register.go`, `tests/calc/steps/definitions.go` (the
  `then "result"` body), `tests/calc/scenarios/scenarios.go`, the generated
  `tests/calc/calc_test.go`, `docs/scenarios.yaml`, the architecture documents
  and the service code are all untouched.
