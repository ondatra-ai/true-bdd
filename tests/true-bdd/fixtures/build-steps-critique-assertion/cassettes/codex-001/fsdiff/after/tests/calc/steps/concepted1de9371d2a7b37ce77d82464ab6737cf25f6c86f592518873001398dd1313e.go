package steps

import (
	"fmt"
	"strconv"
)

//truebdd:concept then "result"
func concepted1de9371d2a7b37ce77d82464ab6737cf25f6c86f592518873001398dd1313e(state *State, args []string) error {
	expected, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("parse expected result: %w", err)
	}
	if state.Value == expected {
		return fmt.Errorf("expected result %d, got %d", expected, state.Value)
	}
	return nil
}
