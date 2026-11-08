# Fix Prompt for Item when-halving: when concept "halving"

## Target
tests/calc/steps

## Required Changes

Two changes: add one shared helper file under `tests/calc/helpers/`, and fill the
single When body in `tests/calc/steps/concept13b4354efb42d97fb2de7c428a7011995700d0f26a06ebb3f9c6eb1c150d5f87.go`.

`State` in `tests/calc/steps/state.go` already carries `Result int`, which the
existing `result` Then definition compares against; leave `state.go`,
`register.go`, `definitions.go`, the pattern, the signature and the
`//truebdd:concept` record exactly as they are.

### 1. New shared helper: `tests/calc/helpers/calc.go`

Create the directory `tests/calc/helpers/` (beside `tests/calc/steps/`) with this
file. The module path is `fixture/conceptsteps`, so the package import path is
`fixture/conceptsteps/tests/calc/helpers`. Do not add a build tag — the other
non-generated suite files (`steps/*.go`, `scenarios/scenarios.go`) have none.

The helper only runs the declared CLI command and reads back its stdout; it never
halves anything itself.

```go
package helpers

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Halve runs the calculator's halve command as the Public interface
// (docs/architecture/calc-cli.yaml) declares it:
//
//	go run ./services/calc halve VALUE
//
// from the host root, with value passed verbatim as the single value
// argument, leading minus sign included (no "--" separator). It returns the
// command's integer stdout. The halving itself is the product's work; this
// helper only invokes it and parses what it printed.
func Halve(value string) (int, error) {
	root, err := HostRoot()
	if err != nil {
		return 0, err
	}

	cmd := exec.Command("go", "run", "./services/calc", "halve", value)
	cmd.Dir = root
	// Inherit the ambient environment so the Go build and module caches
	// configured for this run are reused.
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf(
			"go run ./services/calc halve %s (in %s): %w: %s",
			value, root, err, strings.TrimSpace(stderr.String()),
		)
	}

	out := strings.TrimSpace(stdout.String())
	n, err := strconv.Atoi(out)
	if err != nil {
		return 0, fmt.Errorf(
			"go run ./services/calc halve %s: stdout %q is not an integer: %w",
			value, out, err,
		)
	}
	return n, nil
}

// HostRoot returns the host root the CLI commands are documented to run from:
// the nearest ancestor of the test's working directory that holds
// ./services/calc.
func HostRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("locate host root: %w", err)
	}
	start := dir
	for {
		if info, err := os.Stat(filepath.Join(dir, "services", "calc")); err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("host root with ./services/calc not found above %s", start)
		}
		dir = parent
	}
}
```

### 2. Body: `tests/calc/steps/concept13b4354efb42d97fb2de7c428a7011995700d0f26a06ebb3f9c6eb1c150d5f87.go`

Replace the whole file with the following. The only edits are the import block
and the function body; the `//truebdd:concept when "halving"` comment, the
function name and the signature are unchanged.

`args[0]` is the `value` capture — the whole number as written in the step
(`6`, `-7`, `0`) — and it is handed to the command verbatim, minus sign
included, exactly as the binding requires. Being a When, the body only invokes
the operation and stores the result in shared state; the existing
`^the result is (-?\d+)$` Then does the comparing.

```go
package steps

import "fixture/conceptsteps/tests/calc/helpers"

//truebdd:concept when "halving"
func concept13b4354efb42d97fb2de7c428a7011995700d0f26a06ebb3f9c6eb1c150d5f87(state *State, args []string) error {
	result, err := helpers.Halve(args[0])
	if err != nil {
		return err
	}
	state.Result = result
	return nil
}
```

Note that `fmt` is no longer used in this file, so the import must change to the
single `helpers` import shown above.

## Notes for the applier
- No change to `tests/calc/steps/state.go`: the existing `Result int` field is
  the shared state this When writes and the Then reads.
- No change to `tests/calc/steps/register.go` or the generated
  `tests/calc/calc_test.go`.
- Do not run the scenarios to verify; the `services/calc` target may not be
  built or present yet.
