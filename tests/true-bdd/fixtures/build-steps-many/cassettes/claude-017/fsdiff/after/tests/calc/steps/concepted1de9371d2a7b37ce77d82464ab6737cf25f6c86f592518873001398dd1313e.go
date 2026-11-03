package steps

import (
	"fmt"
	"strconv"
)

//truebdd:concept then "result"
func concepted1de9371d2a7b37ce77d82464ab6737cf25f6c86f592518873001398dd1313e(state *State, args []string) error {
	const (
		// command is the double command of the declared Calculator CLI, whose
		// stdout the doubling When kept for this Then to observe.
		command = "double"
		// wantCaptures is the pattern's single capture, the expected whole
		// number as written in the step.
		wantCaptures = 1
	)

	if len(args) != wantCaptures {
		return fmt.Errorf("result: the step gave %d captures, want %d", len(args), wantCaptures)
	}

	// The capture is an optional minus sign followed by digits, so it reads
	// back as the whole number the step wrote. This is the step's own
	// expectation, not anything worked out from the number that was doubled.
	expected, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("result: the step's expected whole number %q is not an integer: %w", args[0], err)
	}

	if !state.Doubled {
		return fmt.Errorf("result: the %s command was not run, so there is no printed whole number to compare with %d",
			command, expected)
	}

	// The observation is the integer the CLI document declares the double
	// command prints on stdout, as the When read and kept it. The comparison
	// is the whole of this step's work: it never doubles anything itself.
	if state.Result != expected {
		return fmt.Errorf("result: the %s command printed %d on stdout, want %d",
			command, state.Result, expected)
	}

	return nil
}
