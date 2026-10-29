package steps

import (
	"fmt"
	"strconv"
)

func phraseed02cd0805872c6dc539a79f13857bcae5921c4b2ff964f4091e2a4a4e97246a(state *State, args []string) error {
	// Then: observe the result the When recorded and compare it with the
	// expected integer the Scenario states. The When already ran the declared
	// `double` command (docs/architecture/calc-cli.yaml) and kept its integer
	// stdout in state.Result, so nothing is run here and no arithmetic is
	// performed — the product computed the result, this step only compares it.
	const (
		phrase   = "the result is {value:number}"
		zeroWord = "no"
	)

	if len(args) != 1 {
		return fmt.Errorf("%s: expected 1 captured argument, got %d", phrase, len(args))
	}
	if !state.ResultSet {
		return fmt.Errorf("%s: no result recorded; a When must double the number first", phrase)
	}

	raw := args[0]
	expected := 0
	if raw != zeroWord {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("the result is %q: not an integer: %w", raw, err)
		}
		expected = parsed
	}

	if state.Result != expected {
		return fmt.Errorf(
			"the result is %d, but the double command printed %d",
			expected, state.Result,
		)
	}

	return nil
}
