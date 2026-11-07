# Fix Prompt for Item then-result: then concept "result"

## Target
tests/calc/steps

## Required Changes

Write only the body of the one listed definition. Keep the package clause, the
`//truebdd:concept then "result"` record, the function name and the
`(state *State, args []string) error` signature exactly as generated. Do not
touch `tests/calc/steps/register.go`, `tests/calc/steps/state.go`, the other two
definitions, the generated tests, the Specification or any service code.

### 1. Add the shared comparison helper to `tests/calc/helpers/helpers.go`

Append this function to the existing `helpers` package file (its current
imports `fmt` and `strconv` already cover it; add nothing to the import block,
and leave `DoubleCommand`, `CalcSourcePath`, `CalcProgramName`, `HostRoot`,
`BuildCalculator` and `RunDouble` unchanged):

```go
// ExpectedNumber reads the whole number a Then step wrote, as the step's
// capture hands it over: an optionally negative integer in decimal. It only
// reads the step's own text and never doubles or otherwise computes anything.
func ExpectedNumber(capture string) (int, error) {
	expected, err := strconv.Atoi(strings.TrimSpace(capture))
	if err != nil {
		return 0, fmt.Errorf("the result step expects a whole number, got %q", capture)
	}
	return expected, nil
}
```

`strings` is already imported in that file, so the import block stays as it is.

### 2. Replace the body in
`tests/calc/steps/concepted1de9371d2a7b37ce77d82464ab6737cf25f6c86f592518873001398dd1313e.go`

Write the whole file exactly as follows:

```go
package steps

import (
	"fmt"

	"fixture/stepsrun/tests/calc/helpers"
)

//truebdd:concept then "result"
func concepted1de9371d2a7b37ce77d82464ab6737cf25f6c86f592518873001398dd1313e(state *State, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("the result Then expects 1 capture (the expected whole number), got %d", len(args))
	}

	expected, err := helpers.ExpectedNumber(args[0])
	if err != nil {
		return err
	}

	if !state.DoubledSet {
		return fmt.Errorf("no %s command output kept: the doubling When must run first", helpers.DoubleCommand)
	}

	if state.Doubled != expected {
		return fmt.Errorf("the calculator's %s command printed %d, want %d",
			helpers.DoubleCommand, state.Doubled, expected)
	}
	return nil
}
```

## Why this is the binding's body
- The binding's single clause — "Compare the kept double command stdout with the
  step's expected whole number" — is the whole body: it reads what the When kept
  and compares, nothing more.
- The kept stdout is `state.Doubled`, the integer the doubling When read back
  from the declared `calc double VALUE` process through `helpers.RunDouble`,
  which already trims the newline the interface document declares
  (`strings.TrimSpace` on stdout) before converting it. The Then therefore
  observes the trimmed stdout exactly as the capture's meaning describes, and
  never re-runs the program.
- `state.DoubledSet` distinguishes a kept zero from a When that never ran, which
  is the field's documented purpose in `State`.
- The step's capture is the only expected value used; the body performs no
  arithmetic and never doubles anything, so the product's result is never
  computed in the suite.
