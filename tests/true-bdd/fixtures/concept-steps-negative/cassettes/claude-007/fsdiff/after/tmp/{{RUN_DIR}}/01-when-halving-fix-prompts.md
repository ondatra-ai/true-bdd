# Fix Prompt for Item when-halving: when concept "halving"

## Target
tests/calc/steps

## Scope
Write only the body of the one `when` definition of concept "halving". Keep the
`//truebdd:concept when "halving"` record, the function name, the signature and the
registration in `tests/calc/steps/register.go` exactly as they are. No shared-state
change is needed: `State` already carries the `Result int` field that the `then`
concept ("result") compares against, so the `when` body only has to fill it.

## Required Changes

### 1. `tests/calc/steps/concept4f10677f07b3766e772c413192194a4a2a356211a56a28755e7d4750a7a86e64.go`

Replace the whole file with the content below. Only the import block and the
function body change.

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
func concept4f10677f07b3766e772c413192194a4a2a356211a56a28755e7d4750a7a86e64(state *State, args []string) error {
	const hostRoot = "../.."

	number, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("halving: step number %q is not a whole number: %w", args[0], err)
	}

	cmd := exec.Command("go", "run", "./services/calc", "halve", strconv.Itoa(number))
	cmd.Dir = hostRoot

	stdout, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf(
				"halving: halve %d exited with %d: %s",
				number,
				exitErr.ExitCode(),
				strings.TrimSpace(string(exitErr.Stderr)),
			)
		}
		return fmt.Errorf("halving: running halve %d failed: %w", number, err)
	}

	halved, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil {
		return fmt.Errorf(
			"halving: halve %d wrote %q on stdout, which is not an integer: %w",
			number,
			string(stdout),
			err,
		)
	}

	state.Result = halved
	return nil
}
```

## Why this body

- **Follows the binding.** The binding says: "Run the declared halve command as a
  process with the step's whole number and keep its integer stdout as the result."
  The body runs the process, passes the single captured number as the command's
  required integer argument, and stores the command's stdout in `state.Result`.
  Nothing more.
- **Uses only the declared Public interface.** `docs/architecture/calc-cli.yaml`
  declares `halve` as `go run ./services/calc halve VALUE` run "from the host root",
  with the halved integer plus a newline on stdout, exit `0` on success and exit `1`
  for invalid arguments. The argv, the `cmd.Dir`, the `TrimSpace` of the trailing
  newline and the non-zero-exit branch each come from that document; no service
  source was consulted.
- **Host root is `../..`.** `tests/calc/scenarios/scenarios.go` reaches the registry
  as `../../docs/scenarios.yaml`, so the test process runs in `tests/calc` and the
  host root is two levels up. `cmd.Dir = hostRoot` is what makes the declared
  `./services/calc` package path resolve.
- **Never computes the product's result.** The body does not halve anything and does
  not round. `strconv.Atoi` on the captured text only reads the step's own input back
  as an integer so it can be passed as the declared integer argument; the only halved
  value in the function is the one the process wrote on stdout.
- **Observation stays in the `then` block.** The body compares nothing against an
  expected value; it only stores. The existing `result` definition in
  `tests/calc/steps/definitions.go` does the comparing, so the two scenarios
  (`calc-001-halve` with `6`/`3` and `calc-002-halve` with `-7`/`-3`) are served by a
  `when` that stores and a `then` that checks.
- **Resolves the reported capture finding.** The engine has already narrowed the
  pattern to `^the (?:\w+) halves (-?\d+)$`, so `args` holds exactly one element, the
  signed number. The body indexes only `args[0]` and never mentions the actor, in any
  branch, including the error messages — the actor word changes nothing about the
  process run, the argument or the stored result, and a step naming another actor
  binds this same definition unchanged.

## Do not change
- `tests/calc/steps/register.go` (pattern and registration are the engine's).
- `tests/calc/steps/state.go` (no new field needed).
- `tests/calc/steps/definitions.go` (the `then` concept's definition).
- `tests/calc/calc_test.go`, `docs/scenarios.yaml`, `docs/architecture/*`, and
  anything under `services/`.
