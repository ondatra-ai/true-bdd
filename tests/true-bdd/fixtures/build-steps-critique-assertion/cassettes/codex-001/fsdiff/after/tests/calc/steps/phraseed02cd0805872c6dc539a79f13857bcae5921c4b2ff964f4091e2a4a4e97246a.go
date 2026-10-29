package steps

import (
	"fmt"
	"strconv"
)

func phraseed02cd0805872c6dc539a79f13857bcae5921c4b2ff964f4091e2a4a4e97246a(state *State, args []string) error {
	expected, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("the result is %q: expected an integer: %w", args[0], err)
	}

	if state.Value == expected {
		return fmt.Errorf("the result is %d, want %d", state.Value, expected)
	}

	return nil
}
