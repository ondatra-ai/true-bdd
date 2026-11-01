package steps

import (
	"fmt"
	"strconv"
)

//truebdd:concept then "result"
func concepted1de9371d2a7b37ce77d82464ab6737cf25f6c86f592518873001398dd1313e(state *State, args []string) error {
	// The binding says this Then compares the kept stdout of the declared
	// double command with the step's expected whole number. The doubling When
	// already ran that command as a process and kept its integer stdout, so
	// this step only reads the capture and observes what was kept; it never
	// doubles anything itself.
	const stepLabel = "the result is"

	if len(args) != 1 {
		return fmt.Errorf("%s: expected 1 capture (the expected whole number), got %d", stepLabel, len(args))
	}
	written := args[0]

	expected, err := strconv.Atoi(written)
	if err != nil {
		return fmt.Errorf("%s %s: the expected whole number %q is not an integer: %w", stepLabel, written, written, err)
	}

	if !state.ResultSet {
		return fmt.Errorf("%s %s: no double command stdout kept; the doubling When must run first", stepLabel, written)
	}

	if state.Result != expected {
		return fmt.Errorf("%s %s: the double command printed %d, not %d", stepLabel, written, state.Result, expected)
	}

	return nil
}
