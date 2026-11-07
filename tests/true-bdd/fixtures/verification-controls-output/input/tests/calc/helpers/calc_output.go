package helpers

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// outputCaptures is how many captures the then "output" step hands a
// definition: the polarity word the step used, and the pattern that is the
// rest of the step.
const outputCaptures = 2

// MatchesWord and DoesNotMatchWord are the two words the then "output"
// step's first capture can hold. They are the step's own wording, and they
// decide whether the definition requires a match or requires none.
const (
	MatchesWord      = "matches"
	DoesNotMatchWord = "does not match"
)

// StdoutNewline is the trailing newline the interface document declares the
// calculator prints after the value, and which the observing step takes off
// before matching the step's pattern.
const StdoutNewline = "\n"

// ErrUnknownPolarity signals a first capture that is neither of the two
// words the step's pattern allows, so a definition never guesses which of
// a match and no match was asked for.
var ErrUnknownPolarity = errors.New("the step names no known match polarity")

// ErrNoOutput signals that nothing was kept for the operation, so there is
// no printed text to match a pattern against. That is a scenario missing
// its when step, not a calculator whose output failed to match.
var ErrNoOutput = errors.New("no calculator output was kept; no operation ran")

// ErrOutputMismatch signals printed text with no match for a pattern the
// step required a match for.
var ErrOutputMismatch = errors.New("the calculator's output has no match for the step's pattern")

// ErrOutputMatched signals printed text with a match for a pattern the step
// required no match for.
var ErrOutputMatched = errors.New("the calculator's output has a match for the step's pattern")

// OutputExpectationFromStep reads the then "output" step's captures — the
// polarity word the step used, and the regular expression taken as the rest
// of the step, verbatim and unquoted — and returns that expression compiled
// together with whether a match is required. It only reads the step's own
// wording; what the calculator printed is settled elsewhere.
func OutputExpectationFromStep(args []string) (pattern *regexp.Regexp, wantMatch bool, err error) {
	if len(args) != outputCaptures {
		return nil, false, fmt.Errorf("%w: got %d, want %d: %v",
			ErrCaptureCount, len(args), outputCaptures, args)
	}

	switch args[0] {
	case MatchesWord:
		wantMatch = true

	case DoesNotMatchWord:
		wantMatch = false

	default:
		return nil, false, fmt.Errorf("%w: %q", ErrUnknownPolarity, args[0])
	}

	pattern, err = regexp.Compile(args[1])
	if err != nil {
		return nil, false, fmt.Errorf("compiling the step's pattern %q: %w", args[1], err)
	}

	return pattern, wantMatch, nil
}

// PrintedOutput is what the calculator printed for the operation, as the
// step reads it: the kept stdout without its trailing newline. Kept stdout
// that is empty means no operation ran, which is refused apart so an absent
// when step is never reported as a pattern that failed to match.
func PrintedOutput(kept string) (string, error) {
	if kept == "" {
		return "", ErrNoOutput
	}

	return strings.TrimSuffix(kept, StdoutNewline), nil
}

// CheckOutputMatch reports the step's verdict on printed text: a match when
// the step required one, no match when the step said does not match. It
// matches and compares only — what a half or a double is stays the
// calculator's business, and nothing here derives the text it checks.
func CheckOutputMatch(printed string, pattern *regexp.Regexp, wantMatch bool) error {
	matched := pattern.MatchString(printed)

	switch {
	case wantMatch && !matched:
		return fmt.Errorf("%w: %q does not match %s", ErrOutputMismatch, printed, pattern)

	case !wantMatch && matched:
		return fmt.Errorf("%w: %q matches %s", ErrOutputMatched, printed, pattern)
	}

	return nil
}

