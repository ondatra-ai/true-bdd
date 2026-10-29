package steps

import (
	"fmt"
	"strconv"
)

func phraseed02cd0805872c6dc539a79f13857bcae5921c4b2ff964f4091e2a4a4e97246a(state *State, args []string) error {
	const (
		phrase    = "the result is {value:number}"
		zeroWord  = "no"
		wantArgs  = 1
		valueSlot = 0
	)

	if len(args) != wantArgs {
		return fmt.Errorf("%s: want %d argument, got %d", phrase, wantArgs, len(args))
	}

	raw := args[valueSlot]

	expected := 0
	if raw != zeroWord {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("%s: value %q is not a number: %w", phrase, raw, err)
		}
		expected = parsed
	}

	if !state.ResultSet {
		return fmt.Errorf("%s: no result was observed; double the number in a When before this Then", phrase)
	}

	// Compare only: state.Result holds the integer stdout of the `double`
	// command exactly as the When block observed it.
	if state.Result != expected {
		return fmt.Errorf("%s: want result %d, got %d", phrase, expected, state.Result)
	}

	return nil
}
