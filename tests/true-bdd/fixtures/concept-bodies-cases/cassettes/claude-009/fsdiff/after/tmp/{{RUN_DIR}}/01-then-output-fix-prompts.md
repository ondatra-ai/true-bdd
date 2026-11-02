# Fix Prompt for Item then-output: then concept "output"

## Target
tests/calc/steps

## Required Changes

### 1. No shared-state change

`tests/calc/steps/state.go` already carries the only field this concept
observes: `State.Output`, the stdout the `when "operation"` step keeps
(`state.Output = string(stdout)`, raw, including its trailing newline).
Leave `state.go` and `register.go` untouched.

### 2. Replace the body of the one definition

File: `tests/calc/steps/concept95123e5878d35715c67b1f901cd0d6e1fcafc896021616abdece460efff6ed7e.go`

Overwrite the whole file with exactly this content. The package clause, the
`//truebdd:concept then "output"` record and the function signature are
unchanged; only the import list and the body differ.

```go
package steps

import (
	"fmt"
	"regexp"
	"strings"
)

//truebdd:concept then "output"
func concept95123e5878d35715c67b1f901cd0d6e1fcafc896021616abdece460efff6ed7e(state *State, args []string) error {
	comparison, pattern := args[0], args[1]

	if state.Output == "" {
		return fmt.Errorf("no output was kept, so the step cannot match %q: the operation step must run first", pattern)
	}

	// The declared commands print the result and a newline, and the binding
	// matches the pattern against the kept stdout without that trailing
	// newline.
	printed := strings.TrimSuffix(state.Output, "\n")

	expression, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("the step's pattern %q is no regular expression: %w", pattern, err)
	}
	matched := expression.MatchString(printed)

	switch comparison {
	case "matches":
		if !matched {
			return fmt.Errorf("the calculator's output %q does not match %q", printed, pattern)
		}
	case "does not match":
		if matched {
			return fmt.Errorf("the calculator's output %q matches %q", printed, pattern)
		}
	default:
		return fmt.Errorf("the step names no declared comparison: %q", comparison)
	}

	return nil
}
```

## Why this body serves the binding

- `args[0]` is the `comparison` capture and `args[1]` the `pattern` capture,
  in the order the definition's captures name them.
- "Match the step's pattern, a regular expression taken as the rest of the
  step, against the kept stdout without its trailing newline" — the body
  compiles `pattern` as a Go regular expression (the Word mapping's technical
  term) and matches it against `strings.TrimSuffix(state.Output, "\n")`.
- "require a match or, when the step says does not match, no match" — the
  `switch` requires the outcome the `comparison` alternative names and fails
  with the printed text and the pattern otherwise.
- The body only observes and compares. It never halves or doubles anything,
  never re-derives the expected text, and never runs a command; the `when`
  definition already called the declared CLI and kept its stdout. That keeps
  the four Scenarios' patterns (`^3$`, `^12$`, `\.`, `^3\.5$`) decided purely
  by what the calculator printed.
- No package-level constant, variable, helper or type is added; every value
  stays inside the function body.
