package steps

import (
	"fmt"
	"strconv"
)

func phraseed02cd0805872c6dc539a79f13857bcae5921c4b2ff964f4091e2a4a4e97246a(state *State, args []string) error {
	const phrase = "the result is {value:number}"
	// The engine's pattern for a number term also accepts the word form of
	// zero, so accept it here exactly as the Given for the same term does.
	const zeroWord = "no"

	if len(args) != 1 {
		return fmt.Errorf("%s: expected 1 argument, got %d", phrase, len(args))
	}

	raw := args[0]
	expected := 0
	if raw != zeroWord {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("%s: %q is not an integer: %w", phrase, raw, err)
		}
		expected = parsed
	}

	if !state.HasResult {
		return fmt.Errorf("%s: no result observed; a When must run the double command first", phrase)
	}

	if state.Result != expected {
		return fmt.Errorf("%s: double printed %d on stdout, want %d", phrase, state.Result, expected)
	}

	return nil
}
