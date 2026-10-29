package steps

import (
	"fmt"
	"strconv"
)

func phrase7c963587b98a9c9fa59fafbd25518c3af0d8d428753a4c888f114e5a06aa3242(state *State, args []string) error {
	// Given: store the number as the argument for the next `double` command
	// run, per the phrase mapping. No arithmetic happens here — the When runs
	// the declared CLI command and the Then compares its stdout.
	const zeroWord = "no"

	if len(args) != 1 {
		return fmt.Errorf("the number is {value:number}: expected 1 captured argument, got %d", len(args))
	}

	raw := args[0]
	number := 0
	if raw != zeroWord {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("the number is %q: not an integer: %w", raw, err)
		}
		number = parsed
	}

	state.Number = number
	state.NumberSet = true

	return nil
}
