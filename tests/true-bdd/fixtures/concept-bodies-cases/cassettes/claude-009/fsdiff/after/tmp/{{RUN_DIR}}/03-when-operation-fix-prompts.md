# Fix Prompt for Item when-operation: when concept "operation"

## Target
tests/calc/steps

## Finding addressed
`pattern_scope` on the retired definition
`concept40523ceb26a25e37e78222ac562623f48c38b52711218d94fe5cadea337eebba`: its
pattern `^the Owner (?:(exactly) )?(halves|doubles) (-?\d+)$` put the optional
`exactly` group in front of the whole `(halves|doubles)` alternation, so it also
bound "the Owner exactly doubles 6" — a step form with no stated meaning — and
the body only rejected it with an error.

The repaired, engine-owned pattern binds exactness to the halving branch alone:

    ^the Owner ((?:exactly )?halves|doubles) (-?\d+)$

with captures `operation` ("halves" | "exactly halves" | "doubles") and
`number`, served by
`conceptcc0034864638b337019c005041b6023fc84395f3cfcb85aa7757c1b208c55d6e` in
`tests/calc/steps/conceptcc0034864638b337019c005041b6023fc84395f3cfcb85aa7757c1b208c55d6e.go`.
The engine writes that registration, signature and `//truebdd:concept` record,
and retires the old skeleton — do not hand-write `tests/calc/steps/register.go`.

## Required Changes

### 1. Body of the re-proposed definition

Write the body of the engine's planned skeleton at
`tests/calc/steps/conceptcc0034864638b337019c005041b6023fc84395f3cfcb85aa7757c1b208c55d6e.go`.
Keep the `//truebdd:concept when "operation"` record, the function name and the
`func(state *State, args []string) error` signature exactly as the engine wrote
them; replace only the body and the import block. Full intended file:

```go
package steps

import (
	"fixture/conceptbodiescases/tests/calc/helpers"
)

//truebdd:concept when "operation"
func conceptcc0034864638b337019c005041b6023fc84395f3cfcb85aa7757c1b208c55d6e(state *State, args []string) error {
	operation, number := args[0], args[1]

	// The step's operation phrase names which declared command to run and
	// whether the declared halve command gets its --exact flag; the phrase
	// "exactly doubles" can no longer reach here, so no body-side rejection
	// of an unmeant step form remains.
	command, flags, err := helpers.Operation(operation)
	if err != nil {
		return err
	}

	// Run the named command as a process with the step's whole number as its
	// VALUE argument and keep its stdout as the output. The calculator alone
	// halves or doubles; the body neither computes nor interprets the result.
	output, err := helpers.Run(command, append(flags, number)...)
	if err != nil {
		return err
	}

	state.Output = output
	return nil
}
```

Notes for the applier:
- `fmt` is no longer imported by this file: the only `fmt` use in the old body
  was the "takes no --exact flag" rejection of "exactly doubles", which the
  repaired pattern makes unreachable.
- `args[0]` is the whole operation phrase (including any `exactly ` prefix) and
  `args[1]` the number as written, sign included; pass the number through
  verbatim, with no parsing or normalisation.

### 2. Retire the superseded skeleton

Delete `tests/calc/steps/concept40523ceb26a25e37e78222ac562623f48c38b52711218d94fe5cadea337eebba.go`
if the engine's pattern repair leaves it behind. It is the same `when`
"operation" definition under the retired pattern, it is no longer registered,
and it calls `helpers.Command`, which step 3 replaces — leaving it would keep
the unmeant-step rejection in the suite and break the build.

### 3. Shared helpers: `tests/calc/helpers/calc.go`

Replace the `Command` helper with an `Operation` helper that translates the
step's whole operation phrase into the declared command plus the flags that
command declares. Nothing else in the file changes (`ServicePackage`,
`ExactFlag`, `HalveCommand`, `DoubleCommand`, `HostRoot`, `Run` stay as they
are). Replace exactly this block:

```go
// Command returns the declared CLI command for the operation word a step
// names. It only translates the step's word into the declared command name;
// the calculator alone works out what the command prints.
func Command(operation string) (string, error) {
	switch operation {
	case "halves":
		return HalveCommand, nil
	case "doubles":
		return DoubleCommand, nil
	default:
		return "", fmt.Errorf("the declared calculator CLI has no command for the operation %q", operation)
	}
}
```

with:

```go
// Operation returns the declared CLI command, and the flags that command
// declares, for the operation phrase a step names. "halves" and "exactly
// halves" both name the declared halve command, the second with the --exact
// flag halve alone declares; "doubles" names the declared double command,
// which declares no flag. The helper only translates the step's phrase into
// the declared invocation; the calculator alone works out what it prints.
func Operation(operation string) (string, []string, error) {
	switch operation {
	case "halves":
		return HalveCommand, nil, nil
	case "exactly halves":
		return HalveCommand, []string{ExactFlag}, nil
	case "doubles":
		return DoubleCommand, nil, nil
	default:
		return "", nil, fmt.Errorf("the declared calculator CLI has no command for the operation %q", operation)
	}
}
```

Keep the `fmt` import in `tests/calc/helpers/calc.go`: `Operation` and `Run`
still use it. `Operation` returns a freshly built flag slice on every call, so
the caller's `append(flags, number)` cannot alias a shared array.

### 4. Shared state

`tests/calc/steps/state.go` is unchanged: `State{Output string}` already holds
the stdout this `when` keeps for the `then` concept to observe.

## Binding check
- Runs the declared `halve` or `double` command named by the step as a process,
  from the host root via `go run ./services/calc <command> ...`, exactly as
  `docs/architecture/calc-cli.yaml` declares.
- Adds `--exact` precisely when the step's phrase is "exactly halves"; the
  repaired pattern admits the flag only on the halving branch, matching both
  the concept's means and the CLI, which declares `exact` under `halve` only.
- Passes the step's whole number as the command's `VALUE` argument, as written.
- Keeps stdout verbatim in `state.Output`; the body never halves, doubles,
  rounds or compares anything itself.
